// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package node

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/miniBamboo/workshare/workshare"
)

type Master struct {
	PrivateKey  *ecdsa.PrivateKey
	Beneficiary *workshare.Address
}

func (m *Master) Address() workshare.Address {
	return workshare.Address(crypto.PubkeyToAddress(m.PrivateKey.PublicKey))
}
