package poa

import (
	"github.com/miniBamboo/workshare/consensus/builtin"
	"github.com/miniBamboo/workshare/consensus/builtin/authority"
	"github.com/miniBamboo/workshare/state"
	"github.com/miniBamboo/workshare/workshare"
)

// Candidates holds candidates list in memory, and tends to be reused in PoA stage without querying from contract.
type Candidates struct {
	list       []*auworkshareity.Candidate
	masters    map[workshare.Address]int  // map master address to list index
	endorsors  map[workshare.Address]bool // endorsor bitset
	satisfied  []int
	referenced bool
}

// NewCandidates creates candidates list.
func NewCandidates(list []*auworkshareity.Candidate) *Candidates {
	masters := make(map[workshare.Address]int)
	endorsors := make(map[workshare.Address]bool)

	// enable fast check address role
	for i, c := range list {
		masters[c.NodeMaster] = i
		endorsors[c.Endorsor] = true
	}

	return &Candidates{
		list,
		masters,
		endorsors,
		nil,
		false,
	}
}

// Copy make a copy.
func (c *Candidates) Copy() *Candidates {
	c.referenced = true
	copy := *c
	return &copy
}

// Pick picks a list of proposers, which satisfy preset conditions.
func (c *Candidates) Pick(state *state.State) ([]Proposer, error) {
	satisfied := c.satisfied
	if len(satisfied) == 0 {
		// re-pick
		endorsement, err := builtin.Params.Native(state).Get(workshare.KeyProposerEndorsement)
		if err != nil {
			return nil, err
		}

		satisfied = make([]int, 0, len(c.list))
		for i := 0; i < len(c.list) && uint64(len(satisfied)) < workshare.MaxBlockProposers; i++ {
			bal, err := state.GetBalance(c.list[i].Endorsor)
			if err != nil {
				return nil, err
			}
			if bal.Cmp(endorsement) >= 0 {
				satisfied = append(satisfied, i)
			}
		}
		c.satisfied = satisfied
	}

	proposers := make([]Proposer, 0, len(satisfied))
	for _, i := range satisfied {
		proposers = append(proposers, Proposer{
			Address: c.list[i].NodeMaster,
			Active:  c.list[i].Active,
		})
	}
	return proposers, nil
}

// Update update candidate activity status, by its master address.
// It returns false if the given address is not a master.
func (c *Candidates) Update(addr workshare.Address, active bool) bool {
	if i, exist := c.masters[addr]; exist {
		// something like COW
		if c.referenced {
			// shallow copy the list
			c.list = append([]*auworkshareity.Candidate(nil), c.list...)
			c.referenced = false
		}
		copy := *c.list[i]
		copy.Active = active
		c.list[i] = &copy
		return true
	}
	return false
}

// IsEndorsor returns whether an address is an endorsor.
func (c *Candidates) IsEndorsor(addr workshare.Address) bool {
	return c.endorsors[addr]
}

// InvalidateCache invalidate the result cache of Pick method.
func (c *Candidates) InvalidateCache() {
	c.satisfied = nil
}
