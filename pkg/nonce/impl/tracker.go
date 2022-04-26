package impl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	logger "github.com/rs/zerolog/log"
	noncepkg "github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/wallet"
)

var log = logger.With().Str("component", "nonce").Logger()

// LocalTracker implements a nonce tracker that stores
// nonce and pending txs locally.
type LocalTracker struct {
	currNonce  int64
	chainID    int64
	pendingTxs []noncepkg.PendingTx
	wallet     *wallet.Wallet

	// control attributes
	mu        sync.Mutex
	close     chan struct{}
	closeOnce sync.Once

	// external dependencies
	nonceStore  noncepkg.NonceStore
	chainClient noncepkg.ChainClient

	// configs
	checkInterval      time.Duration
	minBlockChainDepth int
	stuckInterval      time.Duration
}

// NewLocalTracker creates a new local tracker.
func NewLocalTracker(
	ctx context.Context,
	w *wallet.Wallet,
	nonceStore noncepkg.NonceStore,
	chainID int64,
	chainClient noncepkg.ChainClient,
	checkInterval time.Duration,
	minBlockChainDepth int,
	stuckInterval time.Duration,
) (*LocalTracker, error) {
	t := &LocalTracker{
		wallet:      w,
		chainID:     chainID,
		nonceStore:  nonceStore,
		chainClient: chainClient,

		checkInterval:      checkInterval,
		minBlockChainDepth: minBlockChainDepth,
		stuckInterval:      stuckInterval,
	}
	if err := t.initialize(ctx); err != nil {
		return nil, fmt.Errorf("tracker initialization: %s", err)
	}

	ticker := time.NewTicker(t.checkInterval)
	t.close = make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				h, err := t.chainClient.HeaderByNumber(ctx, nil)
				if err != nil {
					log.Error().Err(err).Msg("get chain tip header")
					continue
				}

				//copy to avoid data race
				t.mu.Lock()
				pendingTxs := make([]noncepkg.PendingTx, len(t.pendingTxs))
				copy(pendingTxs, t.pendingTxs)
				t.mu.Unlock()

				for _, pendingTx := range pendingTxs {
					if err := t.checkIfPendingTxWasIncluded(ctx, pendingTx, h); err != nil {
						if err == noncepkg.ErrBlockDiffNotEnough {
							break
						}

						log.Error().
							Str("hash", pendingTx.Hash.Hex()).
							Int64("nonce", pendingTx.Nonce).
							Err(err).
							Msg("check if pending tx was included")
					}
				}
			case <-t.close:
				ticker.Stop()
				return
			}
		}
	}()

	log.Info().
		Str("wallet", t.wallet.Address().Hex()).
		Int64("currentNonce", t.currNonce).
		Msg("initializing tracker")

	return t, nil
}

// GetNonce returns the nonce to be used in the next transaction.
// The call is blocked until the client calls unlock.
// The client should also call registerPendingTx if it managed to submit a transaction sucessuflly.
func (t *LocalTracker) GetNonce(ctx context.Context) (noncepkg.RegisterPendingTx, noncepkg.UnlockTracker, int64) {
	t.mu.Lock()

	nonce := t.currNonce

	// this function adds a pending transaction to its list and updates the nonce
	registerPendingTx := func(pendingHash common.Hash) {
		incrementedNonce := nonce + 1

		if err := t.nonceStore.InsertPendingTx(
			ctx,
			t.chainID,
			t.wallet.Address(),
			incrementedNonce,
			pendingHash); err != nil {
			log.Error().
				Err(err).
				Int64("nonce", nonce).
				Str("hash", pendingHash.Hex()).
				Msg("failed to store pending tx")
		}

		t.pendingTxs = append(t.pendingTxs, noncepkg.PendingTx{Hash: pendingHash, Nonce: nonce})
		t.currNonce = incrementedNonce
	}

	// this function frees the mutex
	unlock := func() {
		t.mu.Unlock()
	}

	return registerPendingTx, unlock, nonce
}

// Close closes the background goroutine.
func (t *LocalTracker) Close() {
	t.closeOnce.Do(func() {
		close(t.close)
	})
}

// GetPendingCount returns the number of pendings txs.
func (t *LocalTracker) GetPendingCount(_ context.Context) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.pendingTxs)
}

func (t *LocalTracker) initialize(ctx context.Context) error {
	// Get the nonce from the network
	networkNonce, err := t.chainClient.PendingNonceAt(ctx, t.wallet.Address())
	if err != nil {
		return fmt.Errorf("get pending nonce at: %s", err)
	}

	// Get pending txs for the address
	pendingTxs, err := t.nonceStore.ListPendingTx(ctx, t.chainID, t.wallet.Address())
	if err != nil {
		return fmt.Errorf("get nonce for tracker initialization: %s", err)
	}

	t.pendingTxs = pendingTxs
	t.currNonce = int64(networkNonce)
	return nil
}

func (t *LocalTracker) checkIfPendingTxWasIncluded(
	ctx context.Context,
	pendingTx noncepkg.PendingTx,
	h *types.Header) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	log.Debug().
		Str("hash", pendingTx.Hash.Hex()).
		Int64("nonce", pendingTx.Nonce).
		Msg("checking pending tx...")

	txReceipt, err := t.chainClient.TransactionReceipt(ctx, pendingTx.Hash)
	if err != nil {
		if time.Since(pendingTx.CreatedAt) > t.stuckInterval {
			log.Error().
				Str("hash", pendingTx.Hash.Hex()).
				Int64("nonce", pendingTx.Nonce).
				Time("createdAt", pendingTx.CreatedAt).
				Msg("pending tx may be stuck")

			return noncepkg.ErrPendingTxMayBeStuck
		}

		return fmt.Errorf("get transaction receipt: %s", err)
	}

	blockDiff := h.Number.Int64() - txReceipt.BlockNumber.Int64()
	if blockDiff < int64(t.minBlockChainDepth) {
		log.Debug().
			Str("hash", pendingTx.Hash.Hex()).
			Int64("nonce", pendingTx.Nonce).
			Int64("blockDiff", blockDiff).
			Int64("headNumber", h.Number.Int64()).
			Int64("blockNumber", txReceipt.BlockNumber.Int64()).
			Msg("block difference is not enough")

		return noncepkg.ErrBlockDiffNotEnough
	}

	if err := t.deletePendingTxByHash(ctx, pendingTx.Hash); err != nil {
		return err
	}

	return nil
}

func (t *LocalTracker) deletePendingTxByHash(ctx context.Context, hash common.Hash) error {
	if err := t.nonceStore.DeletePendingTxByHash(ctx, hash); err != nil {
		return fmt.Errorf("delete pending tx: %s", err)
	}

	var deleteIndex int
	for i, pTx := range t.pendingTxs {
		if pTx.Hash.Hex() == hash.Hex() {
			deleteIndex = i
		}
	}
	t.pendingTxs = append(t.pendingTxs[:deleteIndex], t.pendingTxs[deleteIndex+1:]...)

	return nil
}
