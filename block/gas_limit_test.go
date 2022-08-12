// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package block_test

import (
	"math"
	"testing"

	"github.com/miniBamboo/workshare/block"
	"github.com/miniBamboo/workshare/workshare"
	"github.com/stretchr/testify/assert"
)

func TestGasLimit_IsValid(t *testing.T) {

	tests := []struct {
		gl       uint64
		parentGL uint64
		want     bool
	}{
		{workshare.MinGasLimit, workshare.MinGasLimit, true},
		{workshare.MinGasLimit - 1, workshare.MinGasLimit, false},
		{workshare.MinGasLimit, workshare.MinGasLimit * 2, false},
		{workshare.MinGasLimit * 2, workshare.MinGasLimit, false},
		{workshare.MinGasLimit + workshare.MinGasLimit/workshare.GasLimitBoundDivisor, workshare.MinGasLimit, true},
		{workshare.MinGasLimit*2 + workshare.MinGasLimit/workshare.GasLimitBoundDivisor, workshare.MinGasLimit * 2, true},
		{workshare.MinGasLimit*2 - workshare.MinGasLimit/workshare.GasLimitBoundDivisor, workshare.MinGasLimit * 2, true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, block.GasLimit(tt.gl).IsValid(tt.parentGL))
	}
}

func TestGasLimit_Adjust(t *testing.T) {

	tests := []struct {
		gl    uint64
		delta int64
		want  uint64
	}{
		{workshare.MinGasLimit, 1, workshare.MinGasLimit + 1},
		{workshare.MinGasLimit, -1, workshare.MinGasLimit},
		{math.MaxUint64, 1, math.MaxUint64},
		{workshare.MinGasLimit, int64(workshare.MinGasLimit), workshare.MinGasLimit + workshare.MinGasLimit/workshare.GasLimitBoundDivisor},
		{workshare.MinGasLimit * 2, -int64(workshare.MinGasLimit), workshare.MinGasLimit*2 - (workshare.MinGasLimit*2)/workshare.GasLimitBoundDivisor},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, block.GasLimit(tt.gl).Adjust(tt.delta))
	}
}

func TestGasLimit_Qualify(t *testing.T) {
	tests := []struct {
		gl       uint64
		parentGL uint64
		want     uint64
	}{
		{workshare.MinGasLimit, workshare.MinGasLimit, workshare.MinGasLimit},
		{workshare.MinGasLimit - 1, workshare.MinGasLimit, workshare.MinGasLimit},
		{workshare.MinGasLimit, workshare.MinGasLimit * 2, workshare.MinGasLimit*2 - (workshare.MinGasLimit*2)/workshare.GasLimitBoundDivisor},
		{workshare.MinGasLimit * 2, workshare.MinGasLimit, workshare.MinGasLimit + workshare.MinGasLimit/workshare.GasLimitBoundDivisor},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, block.GasLimit(tt.gl).Qualify(tt.parentGL))
	}
}
