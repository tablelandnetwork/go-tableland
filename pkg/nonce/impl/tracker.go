package impl

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/nonce"
	noncepkg "github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/wallet"
)

var log = logger.With().Str("component", "nonce").Logger()

// ErrBlockDiffNotEnough indicates that the pending block is not old enough.
var ErrBlockDiffNotEnough = errors.New("the block number is not old enough to be considered not pending")

// LocalTracker implements a nonce tracker that stores
// nonce and pending txs locally.
type LocalTracker struct {
	currNonce  int64
	network    nonce.Network
	pendingTxs []noncepkg.PendingTx
	wallet     *wallet.Wallet

	// control attributes
	mu       sync.Mutex
	quit     chan struct{}
	isClosed bool

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
	nonceStore nonce.NonceStore,
	chainClient noncepkg.ChainClient,
	checkInterval time.Duration,
	minBlockChainDepth int,
	stuckInterval time.Duration,
) (*LocalTracker, error) {
	t := &LocalTracker{
		wallet:      w,
		network:     nonce.EthereumNetwork,
		nonceStore:  nonceStore,
		chainClient: chainClient,

		isClosed: false,

		checkInterval:      checkInterval,
		minBlockChainDepth: minBlockChainDepth,
		stuckInterval:      stuckInterval,
	}
	if err := t.initialize(ctx); err != nil {
		return nil, fmt.Errorf("tracker initialization: %s", err)
	}

	ticker := time.NewTicker(t.checkInterval)
	t.quit = make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				h, err := t.chainClient.HeaderByNumber(ctx, nil)
				if err != nil {
					log.Error().Err(err).Msg("get chain tip header")
					continue
				}

				for _, pendingTx := range t.pendingTxs {
					if err := t.checkIfPendingTxWasIncluded(ctx, pendingTx, h); err != nil {
						log.Error().Err(err).Msg("check if pending tx was included")
						if err == ErrBlockDiffNotEnough {
							break
						}
					}
				}
			case <-t.quit:
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
func (t *LocalTracker) GetNonce(ctx context.Context) (nonce.RegisterPendingTx, nonce.UnlockTracker, int64) {
	t.mu.Lock()

	nonce := t.currNonce

	// this function frees the mutex, add a pending transaction to its list, and updates the nonce
	registerPendingTx := func(pendingHash common.Hash) {
		incrementedNonce := nonce + 1

		if err := t.nonceStore.InsertPendingTxAndUpsertNonce(
			ctx,
			t.network,
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

	// this function frees the mutex without incrementing the nonce
	unlock := func() {
		t.mu.Unlock()
	}

	return registerPendingTx, unlock, nonce
}

// Close closes the background goroutine.
func (t *LocalTracker) Close() {
	if t.isClosed {
		return
	}
	close(t.quit)
	t.isClosed = true
}

// GetPendingCount returns the number of pendings txs.
func (t *LocalTracker) GetPendingCount(_ context.Context) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.pendingTxs)
}

func (t *LocalTracker) initialize(ctx context.Context) error {
	// Get the nonce stored locally
	nonce, err := t.nonceStore.GetNonce(ctx, t.network, t.wallet.Address())
	if err != nil {
		return fmt.Errorf("get nonce for tracker initialization: %s", err)
	}

	// Get pending txs for the address
	pendingTxs, err := t.nonceStore.ListPendingTx(ctx, t.network, t.wallet.Address())
	if err != nil {
		return fmt.Errorf("get nonce for tracker initialization: %s", err)
	}

	t.pendingTxs = pendingTxs

	// If the local nonce is zero it may indicate that we have no register of the nonce locally
	if nonce.Nonce == 0 {
		// maybe this is not a fresh address, so we need to figured out the nonce
		// by making a call to the network
		networkNonce, err := t.chainClient.PendingNonceAt(ctx, t.wallet.Address())
		if err != nil {
			return fmt.Errorf("get pending nonce at: %s", err)
		}

		if err := t.nonceStore.UpsertNonce(ctx, t.network, t.wallet.Address(), int64(networkNonce)); err != nil {
			return fmt.Errorf("upsert nonce: %s", err)
		}

		nonce = noncepkg.Nonce{
			Network: noncepkg.EthereumNetwork,
			Nonce:   int64(networkNonce),
			Address: t.wallet.Address(),
		}
	}

	t.currNonce = nonce.Nonce
	return nil
}

func (t *LocalTracker) checkIfPendingTxWasIncluded(
	ctx context.Context,
	pendingTx noncepkg.PendingTx,
	h *types.Header) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// There is nothing to check
	if len(t.pendingTxs) == 0 {
		return nil
	}

	log.Debug().
		Str("hash", pendingTx.Hash.Hex()).
		Int64("nonce", pendingTx.Nonce).
		Msg("checking pending tx...")

	txReceipt, err := t.chainClient.TransactionReceipt(ctx, pendingTx.Hash)
	if err != nil {
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

		return ErrBlockDiffNotEnough
	}

	if err := t.nonceStore.DeletePendingTxByHash(ctx, pendingTx.Hash); err != nil {
		return fmt.Errorf("delete pending tx: %s", err)
	}

	t.pendingTxs = t.pendingTxs[1:]

	if time.Since(pendingTx.CreatedAt) > t.stuckInterval {
		log.Error().
			Str("hash", pendingTx.Hash.Hex()).
			Int64("nonce", pendingTx.Nonce).
			Time("createdAt", pendingTx.CreatedAt).
			Msg("pending tx may be stuck")
	}

	return nil
}
