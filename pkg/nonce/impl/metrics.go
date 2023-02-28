package impl

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

func (t *LocalTracker) initMetrics(chainID tableland.ChainID, addr common.Address) error {
	meter := global.MeterProvider().Meter("tableland")
	t.mBaseLabels = append([]attribute.KeyValue{
		attribute.Int64("chain_id", int64(chainID)),
		attribute.String("wallet_address", addr.String()),
	}, metrics.BaseAttrs...)

	mNonce, err := meter.Int64ObservableGauge("tableland.wallettracker.nonce")
	if err != nil {
		return fmt.Errorf("creating nonce metric: %s", err)
	}
	mPendingTxns, err := meter.Int64ObservableGauge("tableland.wallettracker.pending.txns")
	if err != nil {
		return fmt.Errorf("creating pending txns metric: %s", err)
	}
	mBalance, err := meter.Int64ObservableGauge("tableland.wallettracker.balance.wei")
	if err != nil {
		return fmt.Errorf("creating balance metric: %s", err)
	}
	mTxnConfirmationAttempts, err := meter.Int64ObservableGauge("tableland.wallettracker.txn.confirmation.attempts")
	if err != nil {
		return fmt.Errorf("creating txn confirmation attempts metric: %s", err)
	}
	mEthClientUnhealthy, err := meter.Int64ObservableGauge("tableland.wallettracker.eth.client.unhealthy")
	if err != nil {
		return fmt.Errorf("creating eth client unhealthy metric: %s", err)
	}
	t.mUnconfirmedTxnDeletions, err = meter.Int64Counter("tableland.wallettracker.unconfirmed.txn.deletions")
	if err != nil {
		return fmt.Errorf("creating unconfirmed txn deletions metric: %s", err)
	}
	t.mGasBump, err = meter.Int64Counter("tableland.wallettracker.gas.bumps")
	if err != nil {
		return fmt.Errorf("creating gas bump counter metric: %s", err)
	}

	if _, err = meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			t.mu.Lock()
			defer t.mu.Unlock()
			o.ObserveInt64(mNonce, t.currNonce, t.mBaseLabels...)
			o.ObserveInt64(mPendingTxns, int64(len(t.pendingTxs)), t.mBaseLabels...)
			o.ObserveInt64(mBalance, t.currWeiBalance, t.mBaseLabels...)
			o.ObserveInt64(mTxnConfirmationAttempts, t.txnConfirmationAttempts, t.mBaseLabels...)
			o.ObserveInt64(mEthClientUnhealthy, t.ethClientUnhealthy, t.mBaseLabels...)

			return nil
		}, []instrument.Asynchronous{
			mNonce,
			mPendingTxns,
			mBalance,
			mTxnConfirmationAttempts,
			mEthClientUnhealthy,
		}...); err != nil {
		return fmt.Errorf("registering async metric callback: %s", err)
	}

	return nil
}
