package impl

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/wallet"
)

var log = logger.With().Str("component", "nonce").Logger()

// Network type is a string that indicates the network.
type Network string

// EthereumNetwork is referes to Ethereum.
const EthereumNetwork Network = "eth"

// LocalTracker implements a nonce tracker that stores
// nonce and pending txs locally.
type LocalTracker struct {
	currNonce  int64
	network    Network
	wallet     *wallet.Wallet
	pendingTxs []pendingTx
	sqlstore   sqlstore.SystemStore
	backend    bind.ContractBackend
	mu         sync.Mutex
	quit       chan struct{}
}

type pendingTx struct {
	hash  common.Hash
	nonce int64
}

// NewLocalTracker creates a new local tracker.
func NewLocalTracker(
	ctx context.Context,
	w *wallet.Wallet,
	sqlstore sqlstore.SystemStore,
	backend bind.ContractBackend,
) (*LocalTracker, error) {
	t := &LocalTracker{
		wallet:   w,
		network:  EthereumNetwork,
		sqlstore: sqlstore,
		backend:  backend,
	}
	if err := t.initialize(ctx); err != nil {
		return &LocalTracker{}, fmt.Errorf("tracker initialization: %s", err)
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	t.quit = make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				if err := t.checkIfPendingTxWasIncluded(ctx); err != nil {
					log.Error().Err(err).Msg("check if pending tx was included")
				}
			case <-t.quit:
				ticker.Stop()
				return
			}
		}
	}()

	log.
		Info().
		Str("wallet", t.wallet.Address().Hex()).
		Int64("currentNonce", t.currNonce).
		Msg("initializing tracker")

	return t, nil
}

// GetNonce returns the nonce to be used in the next transaction.
// The call is blocked until the client calls either one of the returning functions (registerPendingTx or unlock).
// The client should call registerPendingTx if it managed to submit a transaction sucessuflly.
// Otherwise, it should call unlock.
func (t *LocalTracker) GetNonce(ctx context.Context) (nonce.RegisterPendingTx, nonce.UnlockTracker, int64) {
	t.mu.Lock()

	nonce := t.currNonce

	// this function frees the mutex, add a pending transaction to its list, and updates the nonce
	registerPendingTx := func(pendingHash common.Hash) {
		defer t.mu.Unlock()

		if err := t.sqlstore.UpsertNonce(ctx, string(t.network), t.wallet.Address(), nonce); err != nil {
			log.
				Error().
				Err(err).
				Int64("nonce", nonce).
				Str("hash", pendingHash.Hex()).
				Msg("failed to update nonce")
		}

		if err := t.sqlstore.InsertPendingTx(ctx, string(t.network), t.wallet.Address(), nonce, pendingHash); err != nil {
			log.
				Error().
				Err(err).
				Int64("nonce", nonce).
				Str("hash", pendingHash.Hex()).
				Msg("failed to store pending tx")
		}
		t.pendingTxs = append(t.pendingTxs, pendingTx{hash: pendingHash, nonce: nonce})
		t.currNonce = nonce + 1
	}

	// this function frees the mutex without incrementing the nonce
	unlock := func() {
		t.mu.Unlock()
	}

	return registerPendingTx, unlock, nonce
}

// Close closes the background goroutine.
func (t *LocalTracker) Close() {
	close(t.quit)
}

// GetPendingCount returns the number of pendings txs.
func (t *LocalTracker) GetPendingCount(_ context.Context) int {
	return len(t.pendingTxs)
}

func (t *LocalTracker) initialize(ctx context.Context) error {
	// Get the nonce stored locally
	nonce, err := t.sqlstore.GetNonce(ctx, string(t.network), t.wallet.Address())
	if err != nil {
		return fmt.Errorf("get nonce for tracker initialization: %s", err)
	}

	// Get pending txs for the address
	pendingTxs, err := t.sqlstore.ListPendingTx(ctx, string(t.network), t.wallet.Address())
	if err != nil {
		return fmt.Errorf("get nonce for tracker initialization: %s", err)
	}

	for _, tx := range pendingTxs {
		t.pendingTxs = append(t.pendingTxs, pendingTx{hash: tx.Hash, nonce: tx.Nonce})
	}

	// If the local nonce is zero it may indicate that we have no register of the nonce locally
	if nonce.Nonce == 0 {
		// maybe this is not a fresh address, so we need to figured out the nonce
		// by making a call to the network
		networkNonce, err := t.backend.PendingNonceAt(ctx, t.wallet.Address())
		if err != nil {
			return fmt.Errorf("get pending nonce at: %s", err)
		}

		if err := t.sqlstore.UpsertNonce(ctx, string(t.network), t.wallet.Address(), int64(networkNonce)); err != nil {
			return fmt.Errorf("upsert nonce: %s", err)
		}

		nonce = sqlstore.Nonce{
			Network: string(EthereumNetwork),
			Nonce:   int64(networkNonce),
			Address: t.wallet.Address(),
		}
	}

	t.currNonce = nonce.Nonce
	return nil
}

func (t *LocalTracker) checkIfPendingTxWasIncluded(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// There is nothing to check
	if len(t.pendingTxs) == 0 {
		return nil
	}

	// We have to process in FIFO order
	pendingTx := t.pendingTxs[0]

	_, isPending, err := t.backend.(ethereum.TransactionReader).TransactionByHash(ctx, pendingTx.hash)
	if err != nil {
		return fmt.Errorf("get transaction receipt (tx = %s): %s", pendingTx.hash.Hex(), err)
	}

	if isPending {
		return errors.New("tx still pending")
	}

	if err := t.sqlstore.DeletePendingTxByHash(ctx, pendingTx.hash); err != nil {
		return fmt.Errorf("delete pending tx: %s", err)
	}

	t.pendingTxs = t.pendingTxs[1:]
	return nil
}
