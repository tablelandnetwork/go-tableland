package system

import (
	"context"

	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

// SystemService defines what system operations can be done.
type SystemService interface {
	GetTableMetadata(context.Context, tableland.TableID) (sqlstore.TableMetadata, error)
	GetTablesByController(context.Context, string) ([]sqlstore.Table, error)
}
