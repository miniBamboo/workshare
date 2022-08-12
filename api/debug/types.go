package debug

import (
	"encoding/json"

	"github.com/miniBamboo/workshare/workshare"
)

type TracerOption struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	// Config specific to given tracer.
	Config json.RawMessage `json:"config"`
}

type StorageRangeOption struct {
	Address   workshare.Address
	KeyStart  string
	MaxResult int
	Target    string
}

type StorageRangeResult struct {
	Storage StorageMap         `json:"storage"`
	NextKey *workshare.Bytes32 `json:"nextKey"` // nil if Storage includes the last key in the trie.
}

type StorageMap map[string]StorageEntry

type StorageEntry struct {
	Key   *workshare.Bytes32 `json:"key"`
	Value *workshare.Bytes32 `json:"value"`
}
