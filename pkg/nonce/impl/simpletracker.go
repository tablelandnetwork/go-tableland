package impl

import (
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/wallet"
)

// SimpleTracker is a nonce tracker for testing purposes.
type SimpleTracker struct {
	wallet *wallet.Wallet
	nonce  int64
	mu     sync.Mutex
}

// NewSimpleTracker returns a Simpler Tracker.
func NewSimpleTracker(w *wallet.Wallet, backend bind.ContractBackend) nonce.NonceTracker {
	return &SimpleTracker{
		wallet: w,
		nonce:  0,
	}
}

// GetNonce returne the nonce to be used in the next transaction.
func (t *SimpleTracker) GetNonce(ctx context.Context) (nonce.RegisterPendingTx, nonce.UnlockTracker, int64) {
	t.mu.Lock()

	nonce := t.nonce
	return func(pendingHash common.Hash) {
			t.nonce = t.nonce + 1
			t.mu.Unlock()
		}, func() {
			t.mu.Unlock()
		}, nonce
}

// GetPendingCount returns the number of pendings txs.
func (t *SimpleTracker) GetPendingCount(ctx context.Context) int {
	return 0
}
