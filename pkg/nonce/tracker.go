package nonce

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// PendingTx represents a pending tx.
type PendingTx struct {
	ChainID   int64
	Hash      common.Hash
	Nonce     int64
	Address   common.Address
	CreatedAt time.Time
}

// ErrBlockDiffNotEnough indicates that the pending block is not old enough.
var ErrBlockDiffNotEnough = errors.New("the block number is not old enough to be considered not pending")

// ErrPendingTxMayBeStuck indicates that the pending tx may be stuck.
var ErrPendingTxMayBeStuck = errors.New("pending tx may be stuck")

// ErrReceiptNotFound indicates that the receipt wasn't found.
var ErrReceiptNotFound = errors.New("receipt not found")

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

	// Resync resyncs nonce tracker state with the network.
	// NOTICE: must not call `Resync(..)` if there are still an "open call" to the method `GetNonce(...)`.
	Resync(context.Context) error
}

// ChainClient provides the basic api the a chain needs to provide for an NonceTracker.
type ChainClient interface {
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error)
	BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
	TransactionByHash(ctx context.Context, hash common.Hash) (tx *types.Transaction, isPending bool, err error)
}

// NonceStore provides the api for managing the storage of nonce and pending txs.
type NonceStore interface {
	ListPendingTx(context.Context, common.Address) ([]PendingTx, error)
	InsertPendingTx(context.Context, common.Address, int64, common.Hash) error
	DeletePendingTxByHash(context.Context, common.Hash) error
}
