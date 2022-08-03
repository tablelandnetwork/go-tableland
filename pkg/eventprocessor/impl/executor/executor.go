package executor

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
)

// Executor provides a safe way of executing events contained in an EVM blockchain block.
type Executor interface {
	// NewBlockScope returns a new block scope which can execute events generated by EVM-transactions.
	NewBlockScope(context.Context, int64) (BlockScope, error)

	// GetLastExecutedBlockNumber returns the last executed block number.
	GetLastExecutedBlockNumber(ctx context.Context) (int64, error)

	// Close gracefully closes the executor, waiting for any block scope to be gracefully closed or force closing
	// if the provided context gets canceled.
	Close(context.Context) error
}

// BlockScope provides a sandbox to execute events generated by each EVM transaction in the block.
// It provides an all or nothing execution at the block level, while allowing each transaction processing to also be
// an all or nothing execution of all the events contained in that transaction.
type BlockScope interface {
	// ExecuteTxnEvents executes atomically all the events in an EVM-transaction, returning the TableID where
	// changes were applied. Changes aren't fully committed to the database until Commit(...) is called.
	// If the execution of events in the transaction fails, the client should distinguish between errors of type
	// ErrQueryExecution which aren't recoverable, and infrastructure errors which are recoverable.
	ExecuteTxnEvents(context.Context, eventfeed.TxnEvents) (TxnExecutionResult, error)

	// SetLastprocessedHeight sets a new processed height.
	SetLastProcessedHeight(ctx context.Context, height int64) error

	// SaveTxnReceipts saves a set of transaction receipts.
	SaveTxnReceipts(ctx context.Context, rs []eventprocessor.Receipt) error

	// TxnReceiptExists return true if the provided transaction hash was already processed, and false otherwise.
	TxnReceiptExists(ctx context.Context, txnHash common.Hash) (bool, error)

	// Commit commits all the changes that happened in  previously successful ExecuteTxnEvents(...) calls.
	Commit() error

	// Close gracefully closes the block scope. If Commit(...) called before, it's a noop. If Commit(...) wasn't called,
	// then it will rollback any changes done in previous ExecuteTxnEvents(...) calls.
	Close() error
}

// TxnExecutionResult contains the result of executing a txn with all contained events.
type TxnExecutionResult struct {
	TableID *tableland.TableID

	Error         *string
	ErrorEventIdx *int
}
