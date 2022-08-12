// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package params

import (
	"math/big"
	"testing"

	"github.com/miniBamboo/workshare/muxdb"
	"github.com/miniBamboo/workshare/state"
	"github.com/miniBamboo/workshare/workshare"
	"github.com/stretchr/testify/assert"
)

func TestParamsGetSet(t *testing.T) {
	db := muxdb.NewMem()
	st := state.New(db, workshare.Bytes32{}, 0, 0, 0)
	setv := big.NewInt(10)
	key := workshare.BytesToBytes32([]byte("key"))
	p := New(workshare.BytesToAddress([]byte("par")), st)
	p.Set(key, setv)

	getv, err := p.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, setv, getv)
}
