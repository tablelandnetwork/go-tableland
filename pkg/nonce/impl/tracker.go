package impl

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/nonce"
	noncepkg "github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/wallet"
)

var log = logger.With().Str("component", "nonce").Logger()

// LocalTracker implements a nonce tracker that stores
// nonce and pending txs locally.
type LocalTracker struct {
	currNonce  int64
	network    nonce.Network
	pendingTxs []noncepkg.PendingTx
	wallet     *wallet.Wallet

	// control attributes
	mu   sync.Mutex
	quit chan struct{}

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

		checkInterval:      checkInterval,
		minBlockChainDepth: minBlockChainDepth,
		stuckInterval:      stuckInterval,
	}
	if err := t.initialize(ctx); err != nil {
		return &LocalTracker{}, fmt.Errorf("tracker initialization: %s", err)
	}

	ticker := time.NewTicker(t.checkInterval)
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

		if err := t.nonceStore.UpsertNonce(ctx, t.network, t.wallet.Address(), nonce); err != nil {
			log.Error().
				Err(err).
				Int64("nonce", nonce).
				Str("hash", pendingHash.Hex()).
				Msg("failed to update nonce")
		}

		if err := t.nonceStore.InsertPendingTx(ctx, t.network, t.wallet.Address(), nonce, pendingHash); err != nil {
			log.Error().
				Err(err).
				Int64("nonce", nonce).
				Str("hash", pendingHash.Hex()).
				Msg("failed to store pending tx")
		}
		t.pendingTxs = append(t.pendingTxs, noncepkg.PendingTx{Hash: pendingHash, Nonce: nonce})
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

		if err := t.nonceStore.UpsertNonce(ctx, t.network, t.wallet.Address(), networkNonce); err != nil {
			return fmt.Errorf("upsert nonce: %s", err)
		}

		nonce = noncepkg.Nonce{
			Network: noncepkg.EthereumNetwork,
			Nonce:   networkNonce,
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

	log.Debug().
		Str("hash", pendingTx.Hash.Hex()).
		Int64("nonce", pendingTx.Nonce).
		Msg("checking pending tx...")

	if time.Since(pendingTx.CreatedAt) > t.stuckInterval {
		log.Error().
			Str("hash", pendingTx.Hash.Hex()).
			Int64("nonce", pendingTx.Nonce).
			Time("createdAt", pendingTx.CreatedAt).
			Msg("pending tx may be stuck")
	}

	txReceipt, err := t.chainClient.TransactionReceipt(ctx, pendingTx.Hash)
	if err != nil {
		return fmt.Errorf("get transaction receipt: %s", err)
	}

	h, err := t.chainClient.HeadHeader(ctx)
	if err != nil {
		return fmt.Errorf("get chain tip header: %s", err)
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

		return errors.New("the block number is not old enough to be considered not pending")
	}

	if err := t.nonceStore.DeletePendingTxByHash(ctx, pendingTx.Hash); err != nil {
		return fmt.Errorf("delete pending tx: %s", err)
	}

	t.pendingTxs = t.pendingTxs[1:]
	return nil
}
