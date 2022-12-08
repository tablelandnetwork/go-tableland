package system

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tables"
)

// ErrTableNotFound indicates that the table doesn't exist.
var ErrTableNotFound = errors.New("table not found")

// SystemService defines what system operations can be done.
// TODO(json-rpc): this interface should be cleaned up after dropping support.
type SystemService interface {
	GetTableMetadata(context.Context, tables.TableID) (sqlstore.TableMetadata, error)
	GetTablesByController(context.Context, string) ([]sqlstore.Table, error)
	GetTablesByStructure(context.Context, string) ([]sqlstore.Table, error)
	GetSchemaByTableName(context.Context, string) (sqlstore.TableSchema, error)
	GetReceiptByTransactionHash(context.Context, common.Hash) (sqlstore.Receipt, bool, error)
}
