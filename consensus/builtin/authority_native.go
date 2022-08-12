// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package builtin

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/miniBamboo/workshare/workshare"
	"github.com/miniBamboo/workshare/xenv"
)

func init() {
	defines := []struct {
		name string
		run  func(env *xenv.Environment) []interface{}
	}{
		{"native_executor", func(env *xenv.Environment) []interface{} {
			env.UseGas(workshare.SloadGas)

			val, err := Params.Native(env.State()).Get(workshare.KeyExecutorAddress)
			if err != nil {
				panic(err)
			}

			addr := workshare.BytesToAddress(val.Bytes())
			return []interface{}{addr}
		}},
		{"native_add", func(env *xenv.Environment) []interface{} {
			var args struct {
				NodeMaster common.Address
				Endorsor   common.Address
				Identity   common.Hash
			}
			env.ParseArgs(&args)

			env.UseGas(workshare.SloadGas)
			ok, err := Auworkshareity.Native(env.State()).Add(
				workshare.Address(args.NodeMaster),
				workshare.Address(args.Endorsor),
				workshare.Bytes32(args.Identity))
			if err != nil {
				panic(err)
			}

			if ok {
				env.UseGas(workshare.SstoreSetGas)
				env.UseGas(workshare.SstoreResetGas)
			}
			return []interface{}{ok}
		}},
		{"native_revoke", func(env *xenv.Environment) []interface{} {
			var nodeMaster common.Address
			env.ParseArgs(&nodeMaster)

			env.UseGas(workshare.SloadGas)
			ok, err := Auworkshareity.Native(env.State()).Revoke(workshare.Address(nodeMaster))
			if err != nil {
				panic(err)
			}
			if ok {
				env.UseGas(workshare.SstoreResetGas * 3)
			}
			return []interface{}{ok}
		}},
		{"native_get", func(env *xenv.Environment) []interface{} {
			var nodeMaster common.Address
			env.ParseArgs(&nodeMaster)

			env.UseGas(workshare.SloadGas * 2)
			listed, endorsor, identity, active, err := Auworkshareity.Native(env.State()).Get(workshare.Address(nodeMaster))
			if err != nil {
				panic(err)
			}

			return []interface{}{listed, endorsor, identity, active}
		}},
		{"native_first", func(env *xenv.Environment) []interface{} {
			env.UseGas(workshare.SloadGas)
			nodeMaster, err := Auworkshareity.Native(env.State()).First()
			if err != nil {
				panic(err)
			}
			if nodeMaster != nil {
				return []interface{}{*nodeMaster}
			}
			return []interface{}{workshare.Address{}}
		}},
		{"native_next", func(env *xenv.Environment) []interface{} {
			var nodeMaster common.Address
			env.ParseArgs(&nodeMaster)

			env.UseGas(workshare.SloadGas)
			next, err := Auworkshareity.Native(env.State()).Next(workshare.Address(nodeMaster))
			if err != nil {
				panic(err)
			}
			if next != nil {
				return []interface{}{*next}
			}
			return []interface{}{workshare.Address{}}
		}},
		{"native_isEndorsed", func(env *xenv.Environment) []interface{} {
			var nodeMaster common.Address
			env.ParseArgs(&nodeMaster)

			env.UseGas(workshare.SloadGas * 2)
			listed, endorsor, _, _, err := Auworkshareity.Native(env.State()).Get(workshare.Address(nodeMaster))
			if err != nil {
				panic(err)
			}
			if !listed {
				return []interface{}{false}
			}

			env.UseGas(workshare.GetBalanceGas)
			bal, err := env.State().GetBalance(endorsor)
			if err != nil {
				panic(err)
			}

			env.UseGas(workshare.SloadGas)
			endorsement, err := Params.Native(env.State()).Get(workshare.KeyProposerEndorsement)
			if err != nil {
				panic(err)
			}
			return []interface{}{bal.Cmp(endorsement) >= 0}
		}},
	}
	abi := Auworkshareity.NativeABI()
	for _, def := range defines {
		if method, found := abi.MethodByName(def.name); found {
			nativeMethods[methodKey{Auworkshareity.Address, method.ID()}] = &nativeMethod{
				abi: method,
				run: def.run,
			}
		} else {
			panic("method not found: " + def.name)
		}
	}
}
