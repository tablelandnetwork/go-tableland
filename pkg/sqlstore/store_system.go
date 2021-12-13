package sqlstore

import (
	"context"

	"github.com/google/uuid"
)

// SystemStore defines the methods for interacting with system-wide data
type SystemStore interface {
	InsertTable(ctx context.Context, uuid uuid.UUID, controller string) (err error)
	GetTable(ctx context.Context, uuid uuid.UUID) (Table, error)
}
