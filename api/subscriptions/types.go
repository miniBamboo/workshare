// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package subscriptions

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/miniBamboo/workshare/block"
	"github.com/miniBamboo/workshare/chain"
	"github.com/miniBamboo/workshare/tx"
	"github.com/miniBamboo/workshare/workshare"
)

//BlockMessage block piped by websocket
type BlockMessage struct {
	Number       uint32              `json:"number"`
	ID           workshare.Bytes32   `json:"id"`
	Size         uint32              `json:"size"`
	ParentID     workshare.Bytes32   `json:"parentID"`
	Timestamp    uint64              `json:"timestamp"`
	GasLimit     uint64              `json:"gasLimit"`
	Beneficiary  workshare.Address   `json:"beneficiary"`
	GasUsed      uint64              `json:"gasUsed"`
	TotalScore   uint64              `json:"totalScore"`
	TxsRoot      workshare.Bytes32   `json:"txsRoot"`
	TxsFeatures  uint32              `json:"txsFeatures"`
	StateRoot    workshare.Bytes32   `json:"stateRoot"`
	ReceiptsRoot workshare.Bytes32   `json:"receiptsRoot"`
	Signer       workshare.Address   `json:"signer"`
	Transactions []workshare.Bytes32 `json:"transactions"`
	Obsolete     bool                `json:"obsolete"`
}

func convertBlock(b *chain.ExtendedBlock) (*BlockMessage, error) {
	header := b.Header()
	signer, err := header.Signer()
	if err != nil {
		return nil, err
	}

	txs := b.Transactions()
	txIds := make([]workshare.Bytes32, len(txs))
	for i, tx := range txs {
		txIds[i] = tx.ID()
	}
	return &BlockMessage{
		Number:       header.Number(),
		ID:           header.ID(),
		ParentID:     header.ParentID(),
		Timestamp:    header.Timestamp(),
		TotalScore:   header.TotalScore(),
		GasLimit:     header.GasLimit(),
		GasUsed:      header.GasUsed(),
		Beneficiary:  header.Beneficiary(),
		Signer:       signer,
		Size:         uint32(b.Size()),
		StateRoot:    header.StateRoot(),
		ReceiptsRoot: header.ReceiptsRoot(),
		TxsRoot:      header.TxsRoot(),
		TxsFeatures:  uint32(header.TxsFeatures()),
		Transactions: txIds,
		Obsolete:     b.Obsolete,
	}, nil
}

type LogMeta struct {
	BlockID        workshare.Bytes32 `json:"blockID"`
	BlockNumber    uint32            `json:"blockNumber"`
	BlockTimestamp uint64            `json:"blockTimestamp"`
	TxID           workshare.Bytes32 `json:"txID"`
	TxOrigin       workshare.Address `json:"txOrigin"`
	ClauseIndex    uint32            `json:"clauseIndex"`
}

//TransferMessage transfer piped by websocket
type TransferMessage struct {
	Sender    workshare.Address     `json:"sender"`
	Recipient workshare.Address     `json:"recipient"`
	Amount    *math.HexOrDecimal256 `json:"amount"`
	Meta      LogMeta               `json:"meta"`
	Obsolete  bool                  `json:"obsolete"`
}

func convertTransfer(header *block.Header, tx *tx.Transaction, clauseIndex uint32, transfer *tx.Transfer, obsolete bool) (*TransferMessage, error) {
	origin, err := tx.Origin()
	if err != nil {
		return nil, err
	}

	return &TransferMessage{
		Sender:    transfer.Sender,
		Recipient: transfer.Recipient,
		Amount:    (*math.HexOrDecimal256)(transfer.Amount),
		Meta: LogMeta{
			BlockID:        header.ID(),
			BlockNumber:    header.Number(),
			BlockTimestamp: header.Timestamp(),
			TxID:           tx.ID(),
			TxOrigin:       origin,
			ClauseIndex:    clauseIndex,
		},
		Obsolete: obsolete,
	}, nil
}

//EventMessage event piped by websocket
type EventMessage struct {
	Address  workshare.Address   `json:"address"`
	Topics   []workshare.Bytes32 `json:"topics"`
	Data     string              `json:"data"`
	Meta     LogMeta             `json:"meta"`
	Obsolete bool                `json:"obsolete"`
}

func convertEvent(header *block.Header, tx *tx.Transaction, clauseIndex uint32, event *tx.Event, obsolete bool) (*EventMessage, error) {
	signer, err := tx.Origin()
	if err != nil {
		return nil, err
	}
	return &EventMessage{
		Address: event.Address,
		Data:    hexutil.Encode(event.Data),
		Meta: LogMeta{
			BlockID:        header.ID(),
			BlockNumber:    header.Number(),
			BlockTimestamp: header.Timestamp(),
			TxID:           tx.ID(),
			TxOrigin:       signer,
			ClauseIndex:    clauseIndex,
		},
		Topics:   event.Topics,
		Obsolete: obsolete,
	}, nil
}

// EventFilter contains options for contract event filtering.
type EventFilter struct {
	Address *workshare.Address // restricts matches to events created by specific contracts
	Topic0  *workshare.Bytes32
	Topic1  *workshare.Bytes32
	Topic2  *workshare.Bytes32
	Topic3  *workshare.Bytes32
	Topic4  *workshare.Bytes32
}

// Match returs whether event matches filter
func (ef *EventFilter) Match(event *tx.Event) bool {
	if (ef.Address != nil) && (*ef.Address != event.Address) {
		return false
	}

	matchTopic := func(topic *workshare.Bytes32, index int) bool {
		if topic != nil {
			if len(event.Topics) <= index {
				return false
			}

			if *topic != event.Topics[index] {
				return false
			}
		}
		return true
	}

	return matchTopic(ef.Topic0, 0) &&
		matchTopic(ef.Topic1, 1) &&
		matchTopic(ef.Topic2, 2) &&
		matchTopic(ef.Topic3, 3) &&
		matchTopic(ef.Topic4, 4)
}

// TransferFilter contains options for contract transfer filtering.
type TransferFilter struct {
	TxOrigin  *workshare.Address // who send transaction
	Sender    *workshare.Address // who transferred tokens
	Recipient *workshare.Address // who received tokens
}

// Match returs whether transfer matches filter
func (tf *TransferFilter) Match(transfer *tx.Transfer, origin workshare.Address) bool {
	if (tf.TxOrigin != nil) && (*tf.TxOrigin != origin) {
		return false
	}

	if (tf.Sender != nil) && (*tf.Sender != transfer.Sender) {
		return false
	}

	if (tf.Recipient != nil) && (*tf.Recipient != transfer.Recipient) {
		return false
	}
	return true
}

type BeatMessage struct {
	Number      uint32            `json:"number"`
	ID          workshare.Bytes32 `json:"id"`
	ParentID    workshare.Bytes32 `json:"parentID"`
	Timestamp   uint64            `json:"timestamp"`
	TxsFeatures uint32            `json:"txsFeatures"`
	Bloom       string            `json:"bloom"`
	K           uint32            `json:"k"`
	Obsolete    bool              `json:"obsolete"`
}

type Beat2Message struct {
	Number      uint32            `json:"number"`
	ID          workshare.Bytes32 `json:"id"`
	ParentID    workshare.Bytes32 `json:"parentID"`
	Timestamp   uint64            `json:"timestamp"`
	TxsFeatures uint32            `json:"txsFeatures"`
	GasLimit    uint64            `json:"gasLimit"`
	Bloom       string            `json:"bloom"`
	K           uint8             `json:"k"`
	Obsolete    bool              `json:"obsolete"`
}
