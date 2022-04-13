package nonce

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
)

// RegisterPendingTx registers a pending tx in the nonce tracker.
type RegisterPendingTx func(common.Hash)

// UnlockTracker unlocks the tracker so another thread can call GetNonce.
type UnlockTracker func()

// NonceTracker manages nonce by keeping track of pendings Tx.
type NonceTracker interface {
	// GetNonce returns the nonce to be used in the next transaction.
	// The call is blocked until the client calls either one of the returning functions (registerPendingTx or unlock).
	// The client should call registerPendingTx if it managed to submit a transaction sucessuflly.
	// Otherwise, it should call unlock.
	GetNonce(context.Context) (RegisterPendingTx, UnlockTracker, int64)

	// GetPendingCount returns the number of pendings txs.
	GetPendingCount(context.Context) int
}
