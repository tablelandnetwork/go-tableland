package system

import (
	"context"

	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tables"
)

// SystemService defines what system operations can be done.
type SystemService interface {
	GetTableMetadata(context.Context, tables.TableID) (sqlstore.TableMetadata, error)
	GetTablesByController(context.Context, string) ([]sqlstore.Table, error)
	GetTablesByStructure(context.Context, string) ([]sqlstore.Table, error)
	GetSchemaByTableName(context.Context, string) (sqlstore.TableSchema, error)
}
