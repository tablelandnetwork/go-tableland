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
	wallet  *wallet.Wallet
	backend bind.ContractBackend
	mu      sync.Mutex
}

// NewSimpleTracker returns a Simpler Tracker.
func NewSimpleTracker(w *wallet.Wallet, backend bind.ContractBackend) nonce.NonceTracker {
	return &SimpleTracker{
		wallet:  w,
		backend: backend,
	}
}

// GetNonce returns the nonce to be used in the next transaction.
func (t *SimpleTracker) GetNonce(ctx context.Context) (nonce.RegisterPendingTx, nonce.UnlockTracker, int64) {
	t.mu.Lock()

	nonce, err := t.backend.PendingNonceAt(ctx, t.wallet.Address())
	if err != nil {
		panic(err)
	}
	return func(pendingHash common.Hash) {
			// noop
		}, func() {
			t.mu.Unlock()
		}, int64(nonce)
}

// GetPendingCount returns the number of pendings txs.
func (t *SimpleTracker) GetPendingCount(ctx context.Context) int {
	return 0
}
