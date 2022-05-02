package sqlstore

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v4"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/nonce"
)

// SystemStore defines the methods for interacting with system-wide data.
type SystemStore interface {
	GetTable(context.Context, tableland.TableID) (Table, error)
	GetTablesByController(context.Context, string) ([]Table, error)
	IncrementCreateTableCount(context.Context, string) error
	IncrementRunSQLCount(context.Context, string) error
	GetACLOnTableByController(context.Context, tableland.TableID, string) (SystemACL, error)
	ListPendingTx(context.Context, common.Address) ([]nonce.PendingTx, error)
	InsertPendingTx(context.Context, common.Address, int64, common.Hash) error
	DeletePendingTxByHash(context.Context, common.Hash) error
	WithTx(tx pgx.Tx) SystemStore
	Begin(context.Context) (pgx.Tx, error)
	GetReceipt(context.Context, string) (eventprocessor.Receipt, bool, error)
}
