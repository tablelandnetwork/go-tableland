package nonce

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Network type is a string that indicates the network.
type Network string

// EthereumNetwork is referes to Ethereum.
const EthereumNetwork Network = "eth"

// Nonce represents a nonce for a given address.
type Nonce struct {
	Network Network
	Nonce   int64
	Address common.Address
}

// PendingTx represents a pending tx.
type PendingTx struct {
	Network   Network
	Hash      common.Hash
	Nonce     int64
	Address   common.Address
	CreatedAt time.Time
}

// RegisterPendingTx registers a pending tx in the nonce tracker.
type RegisterPendingTx func(common.Hash)

// UnlockTracker unlocks the tracker so another thread can call GetNonce.
type UnlockTracker func()

// NonceTracker manages nonce by keeping track of pendings Tx.
type NonceTracker interface {
	// GetNonce returns the nonce to be used in the next transaction.
	// The call is blocked until the client calls unlock.
	// The client should also call registerPendingTx if it managed to submit a transaction sucessuflly.
	GetNonce(context.Context) (RegisterPendingTx, UnlockTracker, int64)

	// GetPendingCount returns the number of pendings txs.
	GetPendingCount(context.Context) int
}

// ChainClient provides the basic api the a chain needs to provide for an NonceTracker.
type ChainClient interface {
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error)
}

// NonceStore provides the api for managing the storage of nonce and pending txs.
type NonceStore interface {
	GetNonce(context.Context, Network, common.Address) (Nonce, error)
	UpsertNonce(context.Context, Network, common.Address, int64) error
	ListPendingTx(context.Context, Network, common.Address) ([]PendingTx, error)
	InsertPendingTxAndUpsertNonce(context.Context, Network, common.Address, int64, common.Hash) error
	DeletePendingTxByHash(context.Context, common.Hash) error
}
