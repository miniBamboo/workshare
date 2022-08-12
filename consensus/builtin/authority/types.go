// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package auworkshareity

import (
	"github.com/miniBamboo/workshare/workshare"
)

type (
	entry struct {
		Endorsor workshare.Address
		Identity workshare.Bytes32
		Active   bool
		Prev     *workshare.Address `rlp:"nil"`
		Next     *workshare.Address `rlp:"nil"`
	}

	// Candidate candidate of block proposer.
	Candidate struct {
		NodeMaster workshare.Address
		Endorsor   workshare.Address
		Identity   workshare.Bytes32
		Active     bool
	}
)

// IsEmpty returns whether the entry can be treated as empty.
func (e *entry) IsEmpty() bool {
	return e.Endorsor.IsZero() &&
		e.Identity.IsZero() &&
		!e.Active &&
		e.Prev == nil &&
		e.Next == nil
}

func (e *entry) IsLinked() bool {
	return e.Prev != nil || e.Next != nil
}
