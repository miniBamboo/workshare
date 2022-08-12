// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package builtin

import (
	"encoding/hex"

	"github.com/miniBamboo/workshare/abi"
	"github.com/miniBamboo/workshare/consensus/builtin/gen"
	"github.com/miniBamboo/workshare/workshare"
	"github.com/pkg/errors"
)

type contract struct {
	name    string
	Address workshare.Address
	ABI     *abi.ABI
}

func mustLoadContract(name string) *contract {
	asset := "compiled/" + name + ".abi"
	data := gen.MustAsset(asset)
	abi, err := abi.New(data)
	if err != nil {
		panic(errors.Wrap(err, "load ABI for '"+name+"'"))
	}

	return &contract{
		name,
		workshare.BytesToAddress([]byte(name)),
		abi,
	}
}

// RuntimeBytecodes load runtime byte codes.
func (c *contract) RuntimeBytecodes() []byte {
	asset := "compiled/" + c.name + ".bin-runtime"
	data, err := hex.DecodeString(string(gen.MustAsset(asset)))
	if err != nil {
		panic(errors.Wrap(err, "load runtime byte code for '"+c.name+"'"))
	}
	return data
}

func (c *contract) NativeABI() *abi.ABI {
	asset := "compiled/" + c.name + "Native.abi"
	data := gen.MustAsset(asset)
	abi, err := abi.New(data)
	if err != nil {
		panic(errors.Wrap(err, "load native ABI for '"+c.name+"'"))
	}
	return abi
}
