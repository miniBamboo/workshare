// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package state

import "github.com/miniBamboo/workshare/workshare"

// Stage abstracts changes on the main accounts trie.
type Stage struct {
	root    workshare.Bytes32
	commits []func() error
}

// Hash computes hash of the main accounts trie.
func (s *Stage) Hash() workshare.Bytes32 {
	return s.root
}

// Commit commits all changes into main accounts trie and storage tries.
func (s *Stage) Commit() (root workshare.Bytes32, err error) {
	for _, c := range s.commits {
		if err = c(); err != nil {
			err = &Error{err}
			return
		}
	}
	return s.root, nil
}
