package sqlstore

import "github.com/ethereum/go-ethereum/common"

// Nonce represents a nonce for a given address.
type Nonce struct {
	Network string
	Nonce   int64
	Address common.Address
}

// PendingTx represents a pending tx.
type PendingTx struct {
	Network string
	Hash    common.Hash
	Nonce   int64
	Address common.Address
}
