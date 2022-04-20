package txn

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/parsing"
)

// TxnProcessor executes mutating actions in a Tableland database,
// inside Batches. A Batch allows to run a the actions in an all-or-nothing
// setup, while allowing independent actions to automatically rollback if the
// client wants.
type TxnProcessor interface {
	OpenBatch(context.Context) (Batch, error)
	Close(context.Context) error
}

// Batch is a container for an all-or-nothing execution of mutating actions.
// Each action is executed independently and will automatically rollback if
// fails. The client has the option of rollbacking the whole batch or committing
// the actions that didn't fail.
// Example:
// 1. b := new batch
// 2. b.InsertTable(..) succeeds
// 3. b.ExecWriteQueries(wq1) fails
// 4. b.ExecWriteQueries(wq2) succeeds
// The client has two options:
// a. If Commit() and Closes(), actions 2 and 4 are committed. 3 was automatically rollbacked.
// b. If Closes() (without Commit()), the whole batch will be rollbacked.
//
// The design is targeted for executing a batch of mutating operations in an
// all-or-nothing style, with the extra option of allowing single actions in the batch
// to fail gracefully.
type Batch interface {
	InsertTable(
		ctx context.Context,
		id tableland.TableID,
		controller string,
		description string,
		createStmt parsing.CreateStmt) error
	ExecWriteQueries(ctx context.Context, controller common.Address, wquery []parsing.SugaredMutatingStmt) error

	GetLastProcessedHeight(ctx context.Context) (int64, error)
	SetLastProcessedHeight(ctx context.Context, height int64) error
	SaveTxnReceipts(ctx context.Context, rs []eventprocessor.Receipt) error

	Commit(context.Context) error
	Close(context.Context) error
}

// ErrRowCountExceeded is an error returned when a table exceeds the maximum number
// of rows.
type ErrRowCountExceeded struct {
	BeforeRowCount int
	AfterRowCount  int
}

func (e *ErrRowCountExceeded) Error() string {
	return fmt.Sprintf("table maximum row count exceeded (before %d, after %d)",
		e.BeforeRowCount, e.AfterRowCount)
}

// ErrQueryExecution is an error returned when the query execution failed
// with a cause related to th query itself. Retrying the execution of this query
// will always return an error (e.g: inserting a string in an integer column).
// A query execution failure due to the database being down or any other infrastructure
// problem isn't an ErrQueryExecution error.
type ErrQueryExecution struct {
	Code string
	Msg  string
}

func (e *ErrQueryExecution) Error() string {
	return fmt.Sprintf("query execution failed with code %s: %s",
		e.Code, e.Msg)
}
