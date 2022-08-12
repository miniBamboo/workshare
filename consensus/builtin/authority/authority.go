// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auworkshareity

import (
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/miniBamboo/workshare/state"
	"github.com/miniBamboo/workshare/workshare"
)

var (
	headKey = workshare.Blake2b([]byte("head"))
	tailKey = workshare.Blake2b([]byte("tail"))
)

// Auworkshareity implements native methods of `Auworkshareity` contract.
type Auworkshareity struct {
	addr  workshare.Address
	state *state.State
}

// New create a new instance.
func New(addr workshare.Address, state *state.State) *Auworkshareity {
	return &Auworkshareity{addr, state}
}

func (a *Auworkshareity) getEntry(nodeMaster workshare.Address) (*entry, error) {
	var entry entry
	if err := a.state.DecodeStorage(a.addr, workshare.BytesToBytes32(nodeMaster[:]), func(raw []byte) error {
		if len(raw) == 0 {
			return nil
		}
		return rlp.DecodeBytes(raw, &entry)
	}); err != nil {
		return nil, err
	}
	return &entry, nil
}

func (a *Auworkshareity) setEntry(nodeMaster workshare.Address, entry *entry) error {
	return a.state.EncodeStorage(a.addr, workshare.BytesToBytes32(nodeMaster[:]), func() ([]byte, error) {
		if entry.IsEmpty() {
			return nil, nil
		}
		return rlp.EncodeToBytes(entry)
	})
}

func (a *Auworkshareity) getAddressPtr(key workshare.Bytes32) (addr *workshare.Address, err error) {
	err = a.state.DecodeStorage(a.addr, key, func(raw []byte) error {
		if len(raw) == 0 {
			return nil
		}
		return rlp.DecodeBytes(raw, &addr)
	})
	return
}

func (a *Auworkshareity) setAddressPtr(key workshare.Bytes32, addr *workshare.Address) error {
	return a.state.EncodeStorage(a.addr, key, func() ([]byte, error) {
		if addr == nil {
			return nil, nil
		}
		return rlp.EncodeToBytes(addr)
	})
}

// Get get candidate by node master address.
func (a *Auworkshareity) Get(nodeMaster workshare.Address) (listed bool, endorsor workshare.Address, identity workshare.Bytes32, active bool, err error) {
	var entry *entry
	if entry, err = a.getEntry(nodeMaster); err != nil {
		return
	}
	if entry.IsLinked() {
		return true, entry.Endorsor, entry.Identity, entry.Active, nil
	}
	// if it's the only node, IsLinked will be false.
	// check whether it's the head.
	var ptr *workshare.Address
	if ptr, err = a.getAddressPtr(headKey); err != nil {
		return
	}
	listed = ptr != nil && *ptr == nodeMaster
	return listed, entry.Endorsor, entry.Identity, entry.Active, nil
}

// Add add a new candidate.
func (a *Auworkshareity) Add(nodeMaster workshare.Address, endorsor workshare.Address, identity workshare.Bytes32) (bool, error) {
	entry, err := a.getEntry(nodeMaster)
	if err != nil {
		return false, err
	}
	if !entry.IsEmpty() {
		return false, nil
	}

	entry.Endorsor = endorsor
	entry.Identity = identity
	entry.Active = true // defaults to active

	tailPtr, err := a.getAddressPtr(tailKey)
	if err != nil {
		return false, err
	}
	entry.Prev = tailPtr

	if err := a.setAddressPtr(tailKey, &nodeMaster); err != nil {
		return false, err
	}
	if tailPtr == nil {
		if err := a.setAddressPtr(headKey, &nodeMaster); err != nil {
			return false, err
		}
	} else {
		tailEntry, err := a.getEntry(*tailPtr)
		if err != nil {
			return false, err
		}
		tailEntry.Next = &nodeMaster
		if err := a.setEntry(*tailPtr, tailEntry); err != nil {
			return false, err
		}
	}

	if err := a.setEntry(nodeMaster, entry); err != nil {
		return false, err
	}
	return true, nil
}

// Revoke revoke candidate by given node master address.
// The entry is not removed, but set unlisted and inactive.
func (a *Auworkshareity) Revoke(nodeMaster workshare.Address) (bool, error) {
	entry, err := a.getEntry(nodeMaster)
	if err != nil {
		return false, err
	}
	if !entry.IsLinked() {
		return false, nil
	}

	if entry.Prev == nil {
		if err := a.setAddressPtr(headKey, entry.Next); err != nil {
			return false, err
		}
	} else {
		prevEntry, err := a.getEntry(*entry.Prev)
		if err != nil {
			return false, err
		}
		prevEntry.Next = entry.Next
		if err := a.setEntry(*entry.Prev, prevEntry); err != nil {
			return false, err
		}
	}

	if entry.Next == nil {
		if err := a.setAddressPtr(tailKey, entry.Prev); err != nil {
			return false, err
		}
	} else {
		nextEntry, err := a.getEntry(*entry.Next)
		if err != nil {
			return false, err
		}
		nextEntry.Prev = entry.Prev
		if err := a.setEntry(*entry.Next, nextEntry); err != nil {
			return false, err
		}
	}

	entry.Next = nil
	entry.Prev = nil     // unlist
	entry.Active = false // and set to inactive
	if err := a.setEntry(nodeMaster, entry); err != nil {
		return false, err
	}
	return true, nil
}

// Update update candidate's status.
func (a *Auworkshareity) Update(nodeMaster workshare.Address, active bool) (bool, error) {
	entry, err := a.getEntry(nodeMaster)
	if err != nil {
		return false, err
	}
	if !entry.IsLinked() {
		return false, nil
	}
	entry.Active = active
	if err := a.setEntry(nodeMaster, entry); err != nil {
		return false, err
	}
	return true, nil
}

// Candidates picks a batch of candidates up to limit, that satisfy given endorsement.
func (a *Auworkshareity) Candidates(endorsement *big.Int, limit uint64) ([]*Candidate, error) {
	ptr, err := a.getAddressPtr(headKey)
	if err != nil {
		return nil, err
	}
	candidates := make([]*Candidate, 0, limit)
	for ptr != nil && uint64(len(candidates)) < limit {
		entry, err := a.getEntry(*ptr)
		if err != nil {
			return nil, err
		}
		bal, err := a.state.GetBalance(entry.Endorsor)
		if err != nil {
			return nil, err
		}
		if bal.Cmp(endorsement) >= 0 {
			candidates = append(candidates, &Candidate{
				NodeMaster: *ptr,
				Endorsor:   entry.Endorsor,
				Identity:   entry.Identity,
				Active:     entry.Active,
			})
		}
		ptr = entry.Next
	}
	return candidates, nil
}

// AllCandidates lists all registered candidates.
func (a *Auworkshareity) AllCandidates() ([]*Candidate, error) {
	ptr, err := a.getAddressPtr(headKey)
	if err != nil {
		return nil, err
	}
	var candidates []*Candidate
	for ptr != nil {
		entry, err := a.getEntry(*ptr)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, &Candidate{
			NodeMaster: *ptr,
			Endorsor:   entry.Endorsor,
			Identity:   entry.Identity,
			Active:     entry.Active,
		})
		ptr = entry.Next
	}
	return candidates, nil
}

// First returns node master address of first entry.
func (a *Auworkshareity) First() (*workshare.Address, error) {
	return a.getAddressPtr(headKey)
}

// Next returns address of next node master address after given node master address.
func (a *Auworkshareity) Next(nodeMaster workshare.Address) (*workshare.Address, error) {
	entry, err := a.getEntry(nodeMaster)
	if err != nil {
		return nil, err
	}
	return entry.Next, nil
}
