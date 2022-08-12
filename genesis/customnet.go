// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package genesis

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/miniBamboo/workshare/consensus/builtin"
	"github.com/miniBamboo/workshare/state"
	"github.com/miniBamboo/workshare/tx"
	"github.com/miniBamboo/workshare/workshare"
)

// CustomGenesis is user customized genesis
type CustomGenesis struct {
	LaunchTime     uint64                `json:"launchTime"`
	GasLimit       uint64                `json:"gaslimit"`
	ExtraData      string                `json:"extraData"`
	Accounts       []Account             `json:"accounts"`
	Auworkshareity []Auworkshareity      `json:"auworkshareity"`
	Params         Params                `json:"params"`
	Executor       Executor              `json:"executor"`
	ForkConfig     *workshare.ForkConfig `json:"forkConfig"`
}

// NewCustomNet create custom network genesis.
func NewCustomNet(gen *CustomGenesis) (*Genesis, error) {
	launchTime := gen.LaunchTime

	if gen.GasLimit == 0 {
		gen.GasLimit = workshare.InitialGasLimit
	}
	var executor workshare.Address
	if gen.Params.ExecutorAddress != nil {
		executor = *gen.Params.ExecutorAddress
	} else {
		executor = builtin.Executor.Address
	}

	builder := new(Builder).
		Timestamp(launchTime).
		GasLimit(gen.GasLimit).
		ForkConfig(*gen.ForkConfig).
		State(func(state *state.State) error {
			// alloc builtin contracts
			if err := state.SetCode(builtin.Auworkshareity.Address, builtin.Auworkshareity.RuntimeBytecodes()); err != nil {
				return err
			}
			if err := state.SetCode(builtin.Energy.Address, builtin.Energy.RuntimeBytecodes()); err != nil {
				return err
			}
			if err := state.SetCode(builtin.Extension.Address, builtin.Extension.RuntimeBytecodes()); err != nil {
				return err
			}
			if err := state.SetCode(builtin.Params.Address, builtin.Params.RuntimeBytecodes()); err != nil {
				return err
			}
			if err := state.SetCode(builtin.Prototype.Address, builtin.Prototype.RuntimeBytecodes()); err != nil {
				return err
			}

			if len(gen.Executor.Approvers) > 0 {
				if err := state.SetCode(builtin.Executor.Address, builtin.Executor.RuntimeBytecodes()); err != nil {
					return err
				}
			}

			tokenSupply := &big.Int{}
			energySupply := &big.Int{}
			for _, a := range gen.Accounts {
				if b := (*big.Int)(a.Balance); b != nil {
					if b.Sign() < 0 {
						return fmt.Errorf("%s: balance must be a non-negative integer", a.Address)
					}
					tokenSupply.Add(tokenSupply, b)
					if err := state.SetBalance(a.Address, b); err != nil {
						return err
					}
					if err := state.SetEnergy(a.Address, &big.Int{}, launchTime); err != nil {
						return err
					}
				}
				if e := (*big.Int)(a.Energy); e != nil {
					if e.Sign() < 0 {
						return fmt.Errorf("%s: energy must be a non-negative integer", a.Address)
					}
					energySupply.Add(energySupply, e)
					if err := state.SetEnergy(a.Address, e, launchTime); err != nil {
						return err
					}
				}
				if len(a.Code) > 0 {
					code, err := hexutil.Decode(a.Code)
					if err != nil {
						return fmt.Errorf("invalid contract code for address: %s", a.Address)
					}
					if err := state.SetCode(a.Address, code); err != nil {
						return err
					}
				}
				if len(a.Storage) > 0 {
					for k, v := range a.Storage {
						state.SetStorage(a.Address, workshare.MustParseBytes32(k), v)
					}
				}
			}

			return builtin.Energy.Native(state, launchTime).SetInitialSupply(tokenSupply, energySupply)
		})

	///// initialize builtin contracts

	// initialize params
	bgp := (*big.Int)(gen.Params.BaseGasPrice)
	if bgp != nil {
		if bgp.Sign() < 0 {
			return nil, errors.New("baseGasPrice must be a non-negative integer")
		}
	} else {
		bgp = workshare.InitialBaseGasPrice
	}

	r := (*big.Int)(gen.Params.RewardRatio)
	if r != nil {
		if r.Sign() < 0 {
			return nil, errors.New("rewardRatio must be a non-negative integer")
		}
	} else {
		r = workshare.InitialRewardRatio
	}

	e := (*big.Int)(gen.Params.ProposerEndorsement)
	if e != nil {
		if e.Sign() < 0 {
			return nil, errors.New("proposerEndorsement must a non-negative integer")
		}
	} else {
		e = workshare.InitialProposerEndorsement
	}

	data := mustEncodeInput(builtin.Params.ABI, "set", workshare.KeyExecutorAddress, new(big.Int).SetBytes(executor[:]))
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), workshare.Address{})

	data = mustEncodeInput(builtin.Params.ABI, "set", workshare.KeyRewardRatio, r)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), executor)

	data = mustEncodeInput(builtin.Params.ABI, "set", workshare.KeyBaseGasPrice, bgp)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), executor)

	data = mustEncodeInput(builtin.Params.ABI, "set", workshare.KeyProposerEndorsement, e)
	builder.Call(tx.NewClause(&builtin.Params.Address).WithData(data), executor)

	if len(gen.Auworkshareity) == 0 {
		return nil, errors.New("at least one auworkshareity node")
	}
	// add initial auworkshareity nodes
	for _, anode := range gen.Auworkshareity {
		data := mustEncodeInput(builtin.Auworkshareity.ABI, "add", anode.MasterAddress, anode.EndorsorAddress, anode.Identity)
		builder.Call(tx.NewClause(&builtin.Auworkshareity.Address).WithData(data), executor)
	}

	if len(gen.Executor.Approvers) > 0 {
		// add initial approvers
		for _, approver := range gen.Executor.Approvers {
			data := mustEncodeInput(builtin.Executor.ABI, "addApprover", approver.Address, approver.Identity)
			builder.Call(tx.NewClause(&builtin.Executor.Address).WithData(data), executor)
		}
	}

	if len(gen.ExtraData) > 0 {
		var extra [28]byte
		copy(extra[:], gen.ExtraData)
		builder.ExtraData(extra)
	}

	id, err := builder.ComputeID()
	if err != nil {
		panic(err)
	}
	return &Genesis{builder, id, "customnet"}, nil
}

// Account is the account will set to the genesis block
type Account struct {
	Address workshare.Address            `json:"address"`
	Balance *HexOrDecimal256             `json:"balance"`
	Energy  *HexOrDecimal256             `json:"energy"`
	Code    string                       `json:"code"`
	Storage map[string]workshare.Bytes32 `json:"storage"`
}

// Auworkshareity is the auworkshareity node info
type Auworkshareity struct {
	MasterAddress   workshare.Address `json:"masterAddress"`
	EndorsorAddress workshare.Address `json:"endorsorAddress"`
	Identity        workshare.Bytes32 `json:"identity"`
}

// Executor is the params for executor info
type Executor struct {
	Approvers []Approver `json:"approvers"`
}

// Approver is the approver info for executor contract
type Approver struct {
	Address  workshare.Address `json:"address"`
	Identity workshare.Bytes32 `json:"identity"`
}

// Params means the chain params for params contract
type Params struct {
	RewardRatio         *HexOrDecimal256   `json:"rewardRatio"`
	BaseGasPrice        *HexOrDecimal256   `json:"baseGasPrice"`
	ProposerEndorsement *HexOrDecimal256   `json:"proposerEndorsement"`
	ExecutorAddress     *workshare.Address `json:"executorAddress"`
}

// hexOrDecimal256 marshals big.Int as hex or decimal.
// Copied from go-ethereum/common/math and implement json. Marshaler
type HexOrDecimal256 math.HexOrDecimal256

// UnmarshalJSON implements the json.Unmarshaler interface.
func (i *HexOrDecimal256) UnmarshalJSON(input []byte) error {
	var hex string
	if err := json.Unmarshal(input, &hex); err != nil {
		if err = (*big.Int)(i).UnmarshalJSON(input); err != nil {
			return err
		}
		return nil
	}
	bigint, ok := math.ParseBig256(hex)
	if !ok {
		return fmt.Errorf("invalid hex or decimal integer %q", input)
	}
	*i = HexOrDecimal256(*bigint)
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (i *HexOrDecimal256) MarshalJSON() ([]byte, error) {
	return (*math.HexOrDecimal256)(i).MarshalText()
}
