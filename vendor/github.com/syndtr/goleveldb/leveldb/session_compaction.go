// Copyright (c) 2012, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package leveldb

import (
	"bytes"
	"sort"
	"sync/atomic"

	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/memdb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	undefinedCompaction = iota
	level0Compaction
	nonLevel0Compaction
	seekCompaction
)

func (s *session) pickMemdbLevel(umin, umax []byte, maxLevel int) int {
	v := s.version()
	defer v.release()
	return v.pickMemdbLevel(umin, umax, maxLevel)
}

func (s *session) flushMemdb(rec *sessionRecord, mdb *memdb.DB, maxLevel int) (int, error) {
	// Create sorted table.
	iter := mdb.NewIterator(nil)
	defer iter.Release()
	t, n, err := s.tops.createFrom(iter)
	if err != nil {
		return 0, err
	}

	// Pick level other than zero can cause compaction issue with large
	// bulk insert and delete on strictly incrementing key-space. The
	// problem is that the small deletion markers trapped at lower level,
	// while key/value entries keep growing at higher level. Since the
	// key-space is strictly incrementing it will not overlaps with
	// higher level, thus maximum possible level is always picked, while
	// overlapping deletion marker pushed into lower level.
	// See: https://github.com/syndtr/goleveldb/issues/127.
	flushLevel := s.pickMemdbLevel(t.imin.ukey(), t.imax.ukey(), maxLevel)
	rec.addTableFile(flushLevel, t)

	s.logf("memdb@flush created L%d@%d N·%d S·%s %q:%q", flushLevel, t.fd.Num, n, shortenb(t.size), t.imin, t.imax)
	return flushLevel, nil
}

func (s *session) pickTablesSorted(tables tFiles, sourceLevel int) (*tFile, bool) {
	n := len(tables)
	if oPrefix := s.o.GetOverflowPrefix(); len(oPrefix) > 0 {
		if i := sort.Search(n, func(i int) bool {
			return s.icmp.uCompare(tables[i].imin.ukey(), oPrefix) >= 0
		}); i < n {
			if t := tables[i]; bytes.HasPrefix(t.imin.ukey(), oPrefix) {
				return t, true
			}
		}
	}
	if cptr := s.getCompPtr(sourceLevel); cptr != nil {
		n := len(tables)
		if i := sort.Search(n, func(i int) bool {
			return s.icmp.Compare(tables[i].imax, cptr) > 0
		}); i < n {
			return tables[i], false
		}
	}
	return tables[0], false
}

// Pick a compaction based on current state; need external synchronization.
func (s *session) pickCompaction() *compaction {
	v := s.version()

	var sourceLevel int
	var t0 tFiles
	var typ int
	var overflowed bool
	if v.cScore >= 1 {
		sourceLevel = v.cLevel
		tables := v.levels[sourceLevel]
		if sourceLevel > 0 {
			var picked *tFile
			picked, overflowed = s.pickTablesSorted(tables, sourceLevel)
			t0 = tFiles{picked}
		} else {
			t0 = tFiles{tables[0]}
		}
		if sourceLevel == 0 {
			typ = level0Compaction
		} else {
			typ = nonLevel0Compaction
		}
	} else {
		if p := atomic.LoadPointer(&v.cSeek); p != nil {
			ts := (*tSet)(p)
			sourceLevel = ts.level
			t0 = append(t0, ts.table)
			typ = seekCompaction
		} else {
			v.release()
			return nil
		}
	}

	c := newCompaction(s, v, sourceLevel, t0, typ)
	c.overflowed = overflowed
	return c
}

// Create compaction from given level and range; need external synchronization.
func (s *session) getCompactionRange(sourceLevel int, umin, umax []byte, noLimit bool) *compaction {
	v := s.version()

	if sourceLevel >= len(v.levels) {
		v.release()
		return nil
	}

	t0 := v.levels[sourceLevel].getOverlaps(nil, s.icmp, umin, umax, sourceLevel == 0)
	if len(t0) == 0 {
		v.release()
		return nil
	}

	// Avoid compacting too much in one shot in case the range is large.
	// But we cannot do this for level-0 since level-0 files can overlap
	// and we must not pick one file and drop another older file if the
	// two files overlap.
	if !noLimit && sourceLevel > 0 {
		limit := int64(v.s.o.GetCompactionSourceLimit(sourceLevel))
		total := int64(0)
		for i, t := range t0 {
			total += t.size
			if total >= limit {
				s.logf("table@compaction limiting F·%d -> F·%d", len(t0), i+1)
				t0 = t0[:i+1]
				break
			}
		}
	}

	typ := level0Compaction
	if sourceLevel != 0 {
		typ = nonLevel0Compaction
	}
	return newCompaction(s, v, sourceLevel, t0, typ)
}

func newCompaction(s *session, v *version, sourceLevel int, t0 tFiles, typ int) *compaction {
	c := &compaction{
		s:             s,
		v:             v,
		typ:           typ,
		sourceLevel:   sourceLevel,
		levels:        [2]tFiles{t0, nil},
		maxGPOverlaps: int64(s.o.GetCompactionGPOverlaps(sourceLevel)),
		tPtrs:         make([]int, len(v.levels)),
	}
	c.expand()
	c.reduce()
	c.save()
	return c
}

// compaction represent a compaction state.
type compaction struct {
	s *session
	v *version

	typ           int
	sourceLevel   int
	levels        [2]tFiles
	maxGPOverlaps int64
	skips         tFiles
	skipIndex     int

	gp                tFiles
	gpi               int
	seenKey           bool
	gpOverlappedBytes int64
	imin, imax        internalKey
	tPtrs             []int
	released          bool

	snapGPI               int
	snapSeenKey           bool
	snapGPOverlappedBytes int64
	snapTPtrs             []int
	snapSkipIndex         int

	overflowed bool
}

func (c *compaction) save() {
	c.snapGPI = c.gpi
	c.snapSeenKey = c.seenKey
	c.snapGPOverlappedBytes = c.gpOverlappedBytes
	c.snapTPtrs = append(c.snapTPtrs[:0], c.tPtrs...)
	c.snapSkipIndex = c.skipIndex
}

func (c *compaction) restore() {
	c.gpi = c.snapGPI
	c.seenKey = c.snapSeenKey
	c.gpOverlappedBytes = c.snapGPOverlappedBytes
	c.tPtrs = append(c.tPtrs[:0], c.snapTPtrs...)
	c.skipIndex = c.snapSkipIndex
}

func (c *compaction) release() {
	if !c.released {
		c.released = true
		c.v.release()
	}
}

// Expand compacted tables; need external synchronization.
func (c *compaction) expand() {
	limit := int64(c.s.o.GetCompactionExpandLimit(c.sourceLevel))
	vt0 := c.v.levels[c.sourceLevel]
	vt1 := tFiles{}
	if level := c.sourceLevel + 1; level < len(c.v.levels) {
		vt1 = c.v.levels[level]
	}

	t0, t1 := c.levels[0], c.levels[1]
	imin, imax := t0.getRange(c.s.icmp)

	// For non-zero levels, the ukey can't hop across tables at all.
	if c.sourceLevel == 0 {
		// We expand t0 here just incase ukey hop across tables.
		t0 = vt0.getOverlaps(t0, c.s.icmp, imin.ukey(), imax.ukey(), c.sourceLevel == 0)
		if len(t0) != len(c.levels[0]) {
			imin, imax = t0.getRange(c.s.icmp)
		}
	}
	t1 = vt1.getOverlaps(t1, c.s.icmp, imin.ukey(), imax.ukey(), false)
	// Get entire range covered by compaction.
	amin, amax := append(t0, t1...).getRange(c.s.icmp)

	// See if we can grow the number of inputs in "sourceLevel" without
	// changing the number of "sourceLevel+1" files we pick up.
	if len(t1) > 0 {
		exp0 := vt0.getOverlaps(nil, c.s.icmp, amin.ukey(), amax.ukey(), c.sourceLevel == 0)
		if len(exp0) > len(t0) && t1.size()+exp0.size() < limit {
			xmin, xmax := exp0.getRange(c.s.icmp)
			exp1 := vt1.getOverlaps(nil, c.s.icmp, xmin.ukey(), xmax.ukey(), false)
			if len(exp1) == len(t1) {
				c.s.logf("table@compaction expanding L%d+L%d (F·%d S·%s)+(F·%d S·%s) -> (F·%d S·%s)+(F·%d S·%s)",
					c.sourceLevel, c.sourceLevel+1, len(t0), shortenb(t0.size()), len(t1), shortenb(t1.size()),
					len(exp0), shortenb(exp0.size()), len(exp1), shortenb(exp1.size()))
				imin, imax = xmin, xmax
				t0, t1 = exp0, exp1
				amin, amax = append(t0, t1...).getRange(c.s.icmp)
			}
		}
	}

	// Compute the set of grandparent files that overlap this compaction
	// (parent == sourceLevel+1; grandparent == sourceLevel+2)
	if level := c.sourceLevel + 2; level < len(c.v.levels) {
		c.gp = c.v.levels[level].getOverlaps(c.gp, c.s.icmp, amin.ukey(), amax.ukey(), false)
	}

	c.levels[0], c.levels[1] = t0, t1
	c.imin, c.imax = imin, imax
}

// reduce tries to reduce table set of target level; need external synchronization.
func (c *compaction) reduce() {
	t0, t1 := c.levels[0], c.levels[1]
	if len(t1) <= 2 {
		return
	}

	// Options.
	ro := &opt.ReadOptions{
		DontFillCache: true,
		Strict:        opt.StrictOverride,
	}
	strict := c.s.o.GetStrict(opt.StrictCompaction)
	if strict {
		ro.Strict |= opt.StrictReader
	}

	// the source level iterator
	var it iterator.Iterator
	if c.sourceLevel == 0 {
		its := make([]iterator.Iterator, 0, len(t0))
		for _, t := range t0 {
			its = append(its, c.s.tops.newIterator(t, nil, ro))
		}
		it = iterator.NewMergedIterator(its, c.s.icmp, strict)
	} else {
		it = iterator.NewIndexedIterator(t0.newIndexIterator(c.s.tops, c.s.icmp, nil, ro), strict)
	}
	defer it.Release()

	ft := make(tFiles, 0, len(t1))
	skips := make(tFiles, 0, len(t1)-2)
	targetTableSize := c.s.o.GetCompactionTableSize(c.sourceLevel + 1)
	for i, t := range t1 {
		// the first, the last and small tables are ignored.
		ignore := i == 0 || i == len(t1)-1 || t.size < int64(targetTableSize/10)

		// If no key in source level falls within this table, this table can be safely skipped.
		if !ignore && it.Seek(t.imin) {
			// It's important to compare ukeys here, to prevent ukey from hopping across tables
			// after compaction done.
			if c.s.icmp.uCompare(internalKey(it.Key()).ukey(), t.imax.ukey()) > 0 {
				if it.Prev() {
					if c.s.icmp.uCompare(internalKey(it.Key()).ukey(), t.imin.ukey()) < 0 {
						skips = append(skips, t)
						continue
					}
				}
			}
		}
		ft = append(ft, t)
	}

	if it.Error() == nil && len(skips) > 0 {
		c.s.logf("table@compaction reducing L%d -> L%d F·-%d S·-%s", c.sourceLevel, c.sourceLevel+1, len(skips), shortenb(skips.size()))
		c.levels[1] = ft
		c.skips = skips
	}
}

// Check whether compaction is trivial.
func (c *compaction) trivial() bool {
	return len(c.levels[0]) == 1 && len(c.levels[1]) == 0 && c.gp.size() <= c.maxGPOverlaps
}

func (c *compaction) baseLevelForKey(ukey []byte) bool {
	for level := c.sourceLevel + 2; level < len(c.v.levels); level++ {
		tables := c.v.levels[level]
		for c.tPtrs[level] < len(tables) {
			t := tables[c.tPtrs[level]]
			if c.s.icmp.uCompare(ukey, t.imax.ukey()) <= 0 {
				// We've advanced far enough.
				if c.s.icmp.uCompare(ukey, t.imin.ukey()) >= 0 {
					// Key falls in this file's range, so definitely not base level.
					return false
				}
				break
			}
			c.tPtrs[level]++
		}
	}
	return true
}

func (c *compaction) shouldStopBefore(ikey internalKey) (shouldStop bool) {
	for ; c.gpi < len(c.gp); c.gpi++ {
		gp := c.gp[c.gpi]
		if c.s.icmp.Compare(ikey, gp.imax) <= 0 {
			break
		}
		if c.seenKey {
			c.gpOverlappedBytes += gp.size
		}
	}
	c.seenKey = true

	if c.gpOverlappedBytes > c.maxGPOverlaps {
		// Too much overlap for current output; start new output.
		c.gpOverlappedBytes = 0
		shouldStop = true
	}

	for c.skipIndex < len(c.skips) {
		if c.s.icmp.uCompare(ikey.ukey(), c.skips[c.skipIndex].imin.ukey()) < 0 {
			break
		}
		// hop across the current skipped table; start new output.
		c.skipIndex++
		shouldStop = true
	}
	return
}

// Creates an iterator.
func (c *compaction) newIterator() iterator.Iterator {
	// Creates iterator slice.
	icap := len(c.levels)
	if c.sourceLevel == 0 {
		// Special case for level-0.
		icap = len(c.levels[0]) + 1
	}
	its := make([]iterator.Iterator, 0, icap)

	// Options.
	ro := &opt.ReadOptions{
		DontFillCache: true,
		Strict:        opt.StrictOverride,
	}
	strict := c.s.o.GetStrict(opt.StrictCompaction)
	if strict {
		ro.Strict |= opt.StrictReader
	}

	for i, tables := range c.levels {
		if len(tables) == 0 {
			continue
		}

		// Level-0 is not sorted and may overlaps each other.
		if c.sourceLevel+i == 0 {
			for _, t := range tables {
				its = append(its, c.s.tops.newIterator(t, nil, ro))
			}
		} else {
			it := iterator.NewIndexedIterator(tables.newIndexIterator(c.s.tops, c.s.icmp, nil, ro), strict)
			its = append(its, it)
		}
	}

	return iterator.NewMergedIterator(its, c.s.icmp, strict)
}
