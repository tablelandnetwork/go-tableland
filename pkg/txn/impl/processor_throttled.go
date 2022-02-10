package impl

import (
	"context"
	"time"

	"github.com/textileio/go-tableland/pkg/txn"
)

//  ThrottledTxnProcessor executes mutating actions in a Tableland database.
type ThrottledTxnProcessor struct {
	txnp  txn.TxnProcessor
	delay time.Duration
}

var _ txn.TxnProcessor = (*TblTxnProcessor)(nil)

// NewTxnProcessor returns a new Tableland transaction processor.
func NewThrottledTxnProcessor(txnp txn.TxnProcessor, delay time.Duration) txn.TxnProcessor {
	return &ThrottledTxnProcessor{txnp, delay}
}

// OpenBatch returns a new batch.
func (tp *ThrottledTxnProcessor) OpenBatch(ctx context.Context) (txn.Batch, error) {
	return tp.txnp.OpenBatch(ctx)
}

// Close closes the current opened batch waiting the configured delay.
func (tp *ThrottledTxnProcessor) Close(ctx context.Context) error {
	time.Sleep(tp.delay)
	return tp.txnp.Close(ctx)
}
