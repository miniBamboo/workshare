// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package txpool

import (
	"errors"
	"sync"

	"github.com/miniBamboo/workshare/tx"
	"github.com/miniBamboo/workshare/workshare"
)

// txObjectMap to maintain mapping of tx hash to tx object, and account quota.
type txObjectMap struct {
	lock      sync.RWMutex
	mapByHash map[workshare.Bytes32]*txObject
	mapByID   map[workshare.Bytes32]*txObject
	quota     map[workshare.Address]int
}

func newTxObjectMap() *txObjectMap {
	return &txObjectMap{
		mapByHash: make(map[workshare.Bytes32]*txObject),
		mapByID:   make(map[workshare.Bytes32]*txObject),
		quota:     make(map[workshare.Address]int),
	}
}

func (m *txObjectMap) ContainsHash(txHash workshare.Bytes32) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()
	_, found := m.mapByHash[txHash]
	return found
}

func (m *txObjectMap) Add(txObj *txObject, limitPerAccount int) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	hash := txObj.Hash()
	if _, found := m.mapByHash[hash]; found {
		return nil
	}

	if m.quota[txObj.Origin()] >= limitPerAccount {
		return errors.New("account quota exceeded")
	}

	m.quota[txObj.Origin()]++
	m.mapByHash[hash] = txObj
	m.mapByID[txObj.ID()] = txObj
	return nil
}

func (m *txObjectMap) GetByID(id workshare.Bytes32) *txObject {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.mapByID[id]
}

func (m *txObjectMap) RemoveByHash(txHash workshare.Bytes32) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	if txObj, ok := m.mapByHash[txHash]; ok {
		if m.quota[txObj.Origin()] > 1 {
			m.quota[txObj.Origin()]--
		} else {
			delete(m.quota, txObj.Origin())
		}
		delete(m.mapByHash, txHash)
		delete(m.mapByID, txObj.ID())
		return true
	}
	return false
}

func (m *txObjectMap) ToTxObjects() []*txObject {
	m.lock.RLock()
	defer m.lock.RUnlock()

	txObjs := make([]*txObject, 0, len(m.mapByHash))
	for _, txObj := range m.mapByHash {
		txObjs = append(txObjs, txObj)
	}
	return txObjs
}

func (m *txObjectMap) ToTxs() tx.Transactions {
	m.lock.RLock()
	defer m.lock.RUnlock()

	txs := make(tx.Transactions, 0, len(m.mapByHash))
	for _, txObj := range m.mapByHash {
		txs = append(txs, txObj.Transaction)
	}
	return txs
}

func (m *txObjectMap) Fill(txObjs []*txObject) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, txObj := range txObjs {
		if _, found := m.mapByHash[txObj.Hash()]; found {
			continue
		}
		// skip account limit check

		m.quota[txObj.Origin()]++
		m.mapByHash[txObj.Hash()] = txObj
		m.mapByID[txObj.ID()] = txObj
	}
}

func (m *txObjectMap) Len() int {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return len(m.mapByHash)
}
