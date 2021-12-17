package system

import (
	"context"

	"github.com/google/uuid"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

// SystemService defines what system operations can be done.
type SystemService interface {
	GetTableMetadata(context.Context, uuid.UUID) (sqlstore.TableMetadata, error)
}
