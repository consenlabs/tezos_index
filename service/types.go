package service

import (
	"tezos_index/chain"
	"time"
)

// ordinary transfer result
type OrdinaryTxResult struct {
	Height       int64          `json:"height"`  // block height
	TxHash       string         `json:"tx_hash"` // tx hash
	Sender       string         `json:"sender"`
	Receiver     string         `json:"receiver"`
	Amount       int64          `json:"amount"`
	Fee          int64          `json:"fee"`
	Counter      int64          `json:"nonce"`  // sender nonce
	Status       chain.OpStatus `json:"status"` // tx status
	IsSuccess    bool           `json:"is_success"`
	GasLimit     int64          `json:"gas_limit"`
	GasUsed      int64          `json:"gas_used"`
	GasPrice     float64        `json:"gas_price"`
	StorageLimit int64          `json:"storage_limit"`
	StorageSize  int64          `json:"storage_size"`
	StoragePaid  int64          `json:"storage_paid"`
	Timestamp    time.Time      `json:"time"`
}
