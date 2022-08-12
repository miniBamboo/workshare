// Copyright 2017 The go-ethereum Auworkshares
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package tracers_test

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/miniBamboo/workshare/chain"
	"github.com/miniBamboo/workshare/genesis"
	"github.com/miniBamboo/workshare/muxdb"
	"github.com/miniBamboo/workshare/runtime"
	"github.com/miniBamboo/workshare/state"
	"github.com/miniBamboo/workshare/tracers"
	"github.com/miniBamboo/workshare/tracers/logger"
	"github.com/miniBamboo/workshare/tx"
	"github.com/miniBamboo/workshare/vm"
	"github.com/miniBamboo/workshare/workshare"
	"github.com/miniBamboo/workshare/xenv"
	"github.com/stretchr/testify/assert"

	// Force-load the tracer engines to trigger registration
	_ "github.com/miniBamboo/workshare/tracers/native"
)

type callFrame struct {
	Type    string                `json:"type"`
	From    workshare.Address     `json:"from"`
	To      workshare.Address     `json:"to,omitempty"`
	Value   *math.HexOrDecimal256 `json:"value,omitempty"`
	Gas     math.HexOrDecimal64   `json:"gas"`
	GasUsed math.HexOrDecimal64   `json:"gasUsed"`
	Input   hexutil.Bytes         `json:"input"`
	Output  hexutil.Bytes         `json:"output,omitempty"`
	Error   string                `json:"error,omitempty"`
	Calls   []callFrame           `json:"calls,omitempty"`
}

type clause struct {
	To    *workshare.Address    `json:"to,omitempty"`
	Value *math.HexOrDecimal256 `json:"value"`
	Data  hexutil.Bytes         `json:"data"`
}

type account struct {
	Balance *math.HexOrDecimal256             `json:"balance"`
	Code    hexutil.Bytes                     `json:"code"`
	Nonce   uint64                            `json:"nonce"`
	Storage map[common.Hash]workshare.Bytes32 `json:"storage"`
}

type context struct {
	BlockNumber uint32            `json:"blockNumber"`
	TxOrigin    workshare.Address `json:"txOrigin"`
	ClauseIndex uint32            `json:"clauseIndex"`
	TxID        workshare.Bytes32 `json:"txID"`
}

type traceTest struct {
	State   map[common.Address]account `json:"state,omitempty"`
	Clause  clause                     `json:"clause"`
	Context context                    `json:"context"`
	Calls   callFrame                  `json:"calls,omitempty"`
	Config  json.RawMessage            `json:"config"`
}

type prestate map[common.Address]account

func RunTracerTest(t *testing.T, data *traceTest, tracerName string) json.RawMessage {
	db := muxdb.NewMem()
	gene, _, _, err := genesis.NewTestnet().Build(state.NewStater(db))
	if err != nil {
		t.Fatal(err)
	}

	repo, _ := chain.NewRepository(db, gene)
	st := state.New(db, gene.Header().StateRoot(), 0, 0, 0)
	chain := repo.NewChain(gene.Header().ID())

	for addr, account := range data.State {
		st.SetBalance(workshare.Address(addr), (*big.Int)(account.Balance))
		if len(account.Code) > 0 {
			st.SetCode(workshare.Address(addr), account.Code)
		}
		for k, v := range account.Storage {
			st.SetStorage(workshare.Address(addr), workshare.Bytes32(k), v)
		}
	}

	rt := runtime.New(chain, st, &xenv.BlockContext{
		Number: data.Context.BlockNumber,
	}, workshare.GetForkConfig(gene.Header().ID()))

	var tr tracers.Tracer
	if len(tracerName) > 0 {
		tr, err = tracers.New(tracerName, nil, data.Config)
		assert.Nil(t, err)
	} else {
		cfg, _ := json.Marshal(logger.Config{
			EnableMemory:     true,
			EnableReturnData: true,
		})
		tr, _ = logger.NewStructLogger(cfg)
	}

	rt.SetVMConfig(vm.Config{
		Debug:  true,
		Tracer: tr,
	})

	clause := tx.NewClause(data.Clause.To).WithValue((*big.Int)(data.Calls.Value)).WithData(data.Clause.Data)
	exec, _ := rt.PrepareClause(clause, data.Context.ClauseIndex, uint64(data.Calls.Gas), &xenv.TransactionContext{
		Origin: data.Context.TxOrigin,
		ID:     data.Context.TxID,
	})
	_, _, err = exec()
	assert.Nil(t, err)
	result, err := tr.GetResult()
	assert.Nil(t, err)
	return result
}

func TestNewTracer(t *testing.T) {
	_, err := tracers.New("callTracer", nil, nil)
	assert.Nil(t, err)
}

func TestTracers(t *testing.T) {
	files, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		f := file
		t.Run(strings.TrimSuffix(f.Name(), ".json"), func(t *testing.T) {
			var testData traceTest

			if blob, err := os.ReadFile(filepath.Join("testdata", file.Name())); err != nil {
				t.Fatalf("failed to read testcase: %v", err)
			} else if err := json.Unmarshal(blob, &testData); err != nil {
				t.Fatalf("failed to parse testcase: %v", err)
			}

			result := RunTracerTest(t, &testData, "callTracer")
			var got callFrame
			if err := json.Unmarshal(result, &got); err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, testData.Calls, got)

			result = RunTracerTest(t, &testData, "prestateTracer")
			var pre prestate
			if err := json.Unmarshal(result, &pre); err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, prestate(testData.State), pre)

			RunTracerTest(t, &testData, "")
			RunTracerTest(t, &testData, "4byteTracer")
		})

	}
}
