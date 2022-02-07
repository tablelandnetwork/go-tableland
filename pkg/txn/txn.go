package txn

import (
	"context"

	"github.com/textileio/go-tableland/internal/tableland"
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
	ExecWriteQueries(ctx context.Context, wquery []parsing.SugaredWriteStmt) error

	Commit(context.Context) error
	Close(context.Context) error
}
