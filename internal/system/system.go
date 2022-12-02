package system

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tables"
)

// TODO(json-rpc): this interface should be cleaned up after dropping support.
// SystemService defines what system operations can be done.
type SystemService interface {
	GetTableMetadata(context.Context, tables.TableID) (sqlstore.TableMetadata, error)
	GetTablesByController(context.Context, string) ([]sqlstore.Table, error)
	GetTablesByStructure(context.Context, string) ([]sqlstore.Table, error)
	GetSchemaByTableName(context.Context, string) (sqlstore.TableSchema, error)
	GetReceiptByTransactionHash(context.Context, common.Hash) (sqlstore.Receipt, bool, error)
}
