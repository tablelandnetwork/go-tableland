package sqlstore

import (
	"context"
	"database/sql"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system/db"
	"github.com/textileio/go-tableland/pkg/tables"
)

// SystemStore defines the methods for interacting with system-wide data.
type SystemStore interface {
	GetTable(context.Context, tables.TableID) (Table, error)
	GetTablesByController(context.Context, string) ([]Table, error)

	GetACLOnTableByController(context.Context, tables.TableID, string) (SystemACL, error)

	ListPendingTx(context.Context, common.Address) ([]nonce.PendingTx, error)
	InsertPendingTx(context.Context, common.Address, int64, common.Hash) error
	DeletePendingTxByHash(context.Context, common.Hash) error
	ReplacePendingTxByHash(context.Context, common.Hash, common.Hash) error

	GetReceipt(context.Context, string) (eventprocessor.Receipt, bool, error)

	GetTablesByStructure(context.Context, string) ([]Table, error)
	GetSchemaByTableName(context.Context, string) (TableSchema, error)

	AreEVMEventsPersisted(context.Context, common.Hash) (bool, error)
	SaveEVMEvents(context.Context, []tableland.EVMEvent) error
	GetEVMEvents(context.Context, common.Hash) ([]tableland.EVMEvent, error)
	GetBlocksMissingExtraInfo(context.Context, *int64) ([]int64, error)
	InsertBlockExtraInfo(context.Context, int64, uint64) error
	GetBlockExtraInfo(context.Context, int64) (tableland.EVMBlockInfo, error)

	GetID(context.Context) (string, error)

	Begin(context.Context) (*sql.Tx, error)
	WithTx(tx *sql.Tx) SystemStore
	Queries() *db.Queries
	Close() error
}
