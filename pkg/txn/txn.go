package txn

import (
	"context"

	"github.com/google/uuid"
)

type TxnProcessor interface {
	OpenBatch(context.Context) (Batch, error)
	Close(context.Context) error
}

type Batch interface {
	RegisterTable(ctx context.Context, uuid uuid.UUID, controller string, tableType string, createStmt string) error
	ExecWriteQueries(ctx context.Context, wquery []string) error

	Commit(context.Context) error
	Close(context.Context) error
}
