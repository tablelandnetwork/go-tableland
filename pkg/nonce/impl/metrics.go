package impl

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/tableland"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

func (t *LocalTracker) initMetrics(chainID tableland.ChainID, addr common.Address) error {
	meter := global.MeterProvider().Meter("tableland")
	baseLabels := []attribute.KeyValue{
		attribute.Int64("chain_id", int64(chainID)),
		attribute.String("wallet_address", addr.String()),
	}

	mNonce, err := meter.AsyncInt64().Gauge("tableland.wallettracker.nonce")
	if err != nil {
		return fmt.Errorf("creating nonce metric: %s", err)
	}
	mPendingTxns, err := meter.AsyncInt64().Gauge("tableland.wallettracker.pending.txns")
	if err != nil {
		return fmt.Errorf("creating pending txns metric: %s", err)
	}
	mBalance, err := meter.AsyncInt64().Gauge("tableland.wallettracker.balance.wei")
	if err != nil {
		return fmt.Errorf("creating balance metric: %s", err)
	}
	mTxnConfirmationAttempts, err := meter.AsyncInt64().Gauge("tableland.wallettracker.txn.confirmation.attempts")
	if err != nil {
		return fmt.Errorf("creating lastconfirmed txn timestamp metric: %s", err)
	}

	if err = meter.RegisterCallback(
		[]instrument.Asynchronous{
			mNonce,
			mPendingTxns,
			mBalance,
			mTxnConfirmationAttempts,
		},
		func(ctx context.Context) {
			t.mu.Lock()
			defer t.mu.Unlock()
			mNonce.Observe(ctx, t.currNonce, baseLabels...)
			mPendingTxns.Observe(ctx, int64(len(t.pendingTxs)), baseLabels...)
			mBalance.Observe(ctx, t.currWeiBalance, baseLabels...)
			mTxnConfirmationAttempts.Observe(ctx, t.txnTxnConfirmationAttempts, baseLabels...)
		}); err != nil {
		return fmt.Errorf("registering async metric callback: %s", err)
	}

	return nil
}
