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
type SystemService interface {
	GetTableMetadata(context.Context, tables.TableID) (sqlstore.TableMetadata, error)
	GetReceiptByTransactionHash(context.Context, common.Hash) (sqlstore.Receipt, bool, error)
}
