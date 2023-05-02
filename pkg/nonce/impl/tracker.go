package impl

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	noncepkg "github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/wallet"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
)

const maxGasBumpAttempts = 3

// LocalTracker implements a nonce tracker that stores
// nonce and pending txs locally.
type LocalTracker struct {
	log     zerolog.Logger
	wallet  *wallet.Wallet
	chainID tableland.ChainID

	mu                      sync.Mutex
	currNonce               int64
	currWeiBalance          int64
	txnConfirmationAttempts int64
	ethClientUnhealthy      int64
	pendingTxs              []noncepkg.PendingTx

	// control attributes
	close     chan struct{}
	closeOnce sync.Once

	// external dependencies
	nonceStore  noncepkg.NonceStore
	chainClient noncepkg.ChainClient

	// configs
	checkInterval      time.Duration
	minBlockChainDepth int
	stuckInterval      time.Duration

	// metrics
	mBaseLabels              []attribute.KeyValue
	mUnconfirmedTxnDeletions instrument.Int64Counter
	mGasBump                 instrument.Int64Counter
}

// NewLocalTracker creates a new local tracker. The provided context is used only for initialization
// logic. For graceful closing, the caller should use the Close() API.
func NewLocalTracker(
	ctx context.Context,
	w *wallet.Wallet,
	nonceStore noncepkg.NonceStore,
	chainID tableland.ChainID,
	chainClient noncepkg.ChainClient,
	checkInterval time.Duration,
	minBlockChainDepth int,
	stuckInterval time.Duration,
) (*LocalTracker, error) {
	log := logger.With().
		Str("component", "nonce").
		Int64("chain_id", int64(chainID)).
		Logger()
	t := &LocalTracker{
		log:         log,
		wallet:      w,
		nonceStore:  nonceStore,
		chainClient: chainClient,
		chainID:     chainID,

		checkInterval:      checkInterval,
		minBlockChainDepth: minBlockChainDepth,
		stuckInterval:      stuckInterval,
	}
	if err := t.initMetrics(chainID, w.Address()); err != nil {
		return nil, fmt.Errorf("init metrics: %s", err)
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
				if err := t.checkPendingTxns(); err != nil {
					log.Error().Err(err).Msg("checking pending txns")
				}
				if err := t.checkBalance(); err != nil {
					log.Error().Err(err).Msg("checking balance")
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
			nonce,
			pendingHash); err != nil {
			t.log.Error().
				Err(err).
				Int64("nonce", nonce).
				Str("hash", pendingHash.Hex()).
				Msg("failed to store pending tx")
		}

		t.pendingTxs = append(t.pendingTxs, noncepkg.PendingTx{Hash: pendingHash, Nonce: nonce, CreatedAt: time.Now()})
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

// Resync resyncs nonce tracker state with the network.
// NOTICE: must not call `Resync(..)` if there are still an "open call" to the method `GetNonce(...)`.
func (t *LocalTracker) Resync(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.initialize(ctx)
}

// initialize needs to be called in the ctor or *already* holding the lock.
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
	h *types.Header,
) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.log.Debug().
		Str("hash", pendingTx.Hash.Hex()).
		Int64("nonce", pendingTx.Nonce).
		Msg("checking pending tx...")

	txReceipt, err := t.chainClient.TransactionReceipt(ctx, pendingTx.Hash)
	if err != nil {
		if time.Since(pendingTx.CreatedAt) > t.stuckInterval {
			t.log.Error().
				Str("hash", pendingTx.Hash.Hex()).
				Int64("nonce", pendingTx.Nonce).
				Time("createdAt", pendingTx.CreatedAt).
				Int64("bumpPriceCount", int64(pendingTx.BumpPriceCount)).
				Msg("pending tx may be stuck")

			_, _, err := t.chainClient.TransactionByHash(ctx, pendingTx.Hash)
			if err != nil && strings.Contains(err.Error(), "not found") {
				t.log.Info().
					Str("hash", pendingTx.Hash.Hex()).
					Int64("nonce", pendingTx.Nonce).
					Time("createdAt", pendingTx.CreatedAt).
					Msg("deleting from pending since isn't in the mempool")
				if err := t.deletePendingTxByHash(ctx, pendingTx.Hash); err != nil {
					return fmt.Errorf("deleting pending tx not present in mempool: %s", err)
				}
				t.mUnconfirmedTxnDeletions.Add(ctx, 1, t.mBaseLabels...)
				// Txn being removed from the mempool can leave an unsynced in-memory nonce, since
				// the next nonce will go backwards. Resync everything to be in a safe spot for the
				// ethereum client sending new txn with the right nonce.
				if err := t.initialize(ctx); err != nil {
					return fmt.Errorf("re-initializing after pending tx removed from the pool: %s", err)
				}
				return nil
			}

			return noncepkg.ErrPendingTxMayBeStuck
		}
		if strings.Contains(err.Error(), "not found") {
			return noncepkg.ErrReceiptNotFound
		}
		return fmt.Errorf("get transaction receipt: %s", err)
	}

	blockDiff := h.Number.Int64() - txReceipt.BlockNumber.Int64()
	if blockDiff < int64(t.minBlockChainDepth) {
		t.log.Debug().
			Str("hash", pendingTx.Hash.Hex()).
			Int64("nonce", pendingTx.Nonce).
			Int64("block_diff", blockDiff).
			Int64("head_number", h.Number.Int64()).
			Int64("block_number", txReceipt.BlockNumber.Int64()).
			Msg("block difference is not enough")

		return noncepkg.ErrBlockDiffNotEnough
	}

	if err := t.deletePendingTxByHash(ctx, pendingTx.Hash); err != nil {
		return fmt.Errorf("deleting pending txn by hash: %s", err)
	}

	return nil
}

func (t *LocalTracker) deletePendingTxByHash(ctx context.Context, hash common.Hash) error {
	if err := t.nonceStore.DeletePendingTxByHash(ctx, t.chainID, hash); err != nil {
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

func (t *LocalTracker) checkPendingTxns() error {
	ctx, cls := context.WithTimeout(context.Background(), time.Second*15)
	defer cls()
	h, err := t.chainClient.HeaderByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("get chain tip header: %s", err)
	}

	// copy to avoid data race
	t.mu.Lock()
	pendingTxs := make([]noncepkg.PendingTx, len(t.pendingTxs))
	copy(pendingTxs, t.pendingTxs)
	t.mu.Unlock()

	for i, pendingTx := range pendingTxs {
		ctx, cls := context.WithTimeout(context.Background(), time.Second*15)
		if err := t.checkIfPendingTxWasIncluded(ctx, pendingTx, h); err != nil {
			if err == noncepkg.ErrBlockDiffNotEnough {
				cls()
				break
			}
			if err == noncepkg.ErrReceiptNotFound {
				t.log.Info().
					Str("hash", pendingTx.Hash.Hex()).
					Int64("nonce", pendingTx.Nonce).
					Msg("receipt not found")
				cls()
				break
			}

			if err == noncepkg.ErrPendingTxMayBeStuck {
				// Did we already bump this txn fees enough times?
				// If that's the case, stop since something more weird can be happening.
				if pendingTx.BumpPriceCount > maxGasBumpAttempts {
					t.txnConfirmationAttempts++
					cls()
					break
				}
				t.txnConfirmationAttempts = 0

				// The pending txn seems to be stuck, and we have quota for bumping
				// the gas prices. Let's do that.
				t.mGasBump.Add(ctx, 1, t.mBaseLabels...)
				bumpedTxnHash, err := t.bumpTxnGas(ctx, pendingTx.Hash)
				if err != nil {
					t.log.Error().
						Str("hash", pendingTx.Hash.Hex()).
						Int64("nonce", pendingTx.Nonce).
						Err(err).
						Msg("bumping pending transaction gas price")
					cls()
					break
				}
				if err := t.nonceStore.ReplacePendingTxByHash(ctx, t.chainID, pendingTx.Hash, bumpedTxnHash); err != nil {
					t.log.Error().
						Str("hash", pendingTx.Hash.Hex()).
						Int64("nonce", pendingTx.Nonce).
						Err(err).
						Msg("replacing pending txn with bumped one in db:")
					cls()
					break
				}
				// Replace the pending txn to the new one in-memory slice.
				pendingTxs[i].Hash = bumpedTxnHash
				pendingTxs[i].BumpPriceCount++
				pendingTxs[i].CreatedAt = time.Now()
				cls()
				break
			}

			t.log.Error().
				Str("hash", pendingTx.Hash.Hex()).
				Int64("nonce", pendingTx.Nonce).
				Err(err).
				Msg("check if pending tx was included")
		}
		cls()
	}
	return nil
}

func (t *LocalTracker) checkBalance() error {
	ctx, cls := context.WithTimeout(context.Background(), time.Second*15)
	defer cls()
	weiBalance, err := t.chainClient.BalanceAt(ctx, t.wallet.Address(), nil)
	if err != nil {
		t.mu.Lock()
		t.ethClientUnhealthy++
		t.mu.Unlock()

		return fmt.Errorf("get balance: %s", err)
	}

	s := weiBalance.String()
	var gWeiBalance int64
	if len(s) > 9 {
		gWeiBalance, err = strconv.ParseInt(s[:len(s)-9], 10, 64)
		if err != nil {
			return fmt.Errorf("converting wei to gwei: %s", err)
		}
	}

	t.mu.Lock()
	t.currWeiBalance = gWeiBalance
	t.ethClientUnhealthy = 0
	t.mu.Unlock()

	return nil
}

func (t *LocalTracker) bumpTxnGas(ctx context.Context, txnHash common.Hash) (common.Hash, error) {
	pendingTxn, isPending, err := t.chainClient.TransactionByHash(ctx, txnHash)
	if err != nil {
		return common.Hash{}, fmt.Errorf("get pending txn from the mempool: %s", err)
	}
	if !isPending {
		return common.Hash{}, fmt.Errorf("the transaction hash %s isn't pending", txnHash)
	}

	candidateGasPriceSuggested, err := t.chainClient.SuggestGasPrice(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("get suggested gas price: %s", err)
	}
	candidateOldGasPricePlus25 := big.NewInt(0).
		Div(big.NewInt(0).Mul(pendingTxn.GasPrice(), big.NewInt(125)), big.NewInt(100))

	newGasPrice := candidateOldGasPricePlus25
	if newGasPrice.Cmp(candidateGasPriceSuggested) < 0 {
		newGasPrice = candidateGasPriceSuggested
	}

	ltxn := &types.LegacyTx{
		Nonce:    pendingTxn.Nonce(),
		GasPrice: newGasPrice,
		Gas:      pendingTxn.Gas(),
		To:       pendingTxn.To(),
		Value:    pendingTxn.Value(),
		Data:     pendingTxn.Data(),
	}
	t.log.Info().
		Int64("old_gas_price", pendingTxn.GasPrice().Int64()).
		Int64("candidate_old_price+25", candidateOldGasPricePlus25.Int64()).
		Int64("candidate_gas_price_suggested", candidateGasPriceSuggested.Int64()).
		Int64("new_decided_gas_price", newGasPrice.Int64()).
		Msg("bumped txn gas price summary")

	signer := types.NewLondonSigner(big.NewInt(int64(t.chainID)))
	txn, err := types.SignTx(types.NewTx(ltxn), signer, t.wallet.PrivateKey())
	if err != nil {
		return common.Hash{}, fmt.Errorf("signing txn: %s", err)
	}

	if err := t.chainClient.SendTransaction(ctx, txn); err != nil {
		return common.Hash{}, fmt.Errorf("sending txn: %s", err)
	}

	return txn.Hash(), nil
}
