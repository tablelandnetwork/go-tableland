package sqlstore

import (
	"context"

	"github.com/google/uuid"
)

// SystemStore defines the methods for interacting with system-wide data.
type SystemStore interface {
	InsertTable(context.Context, uuid.UUID, string, string) (err error)
	GetTable(context.Context, uuid.UUID) (Table, error)
	GetTablesByController(context.Context, string) ([]Table, error)
	Authorize(context.Context, string) error
	Revoke(context.Context, string) error
	IsAuthorized(context.Context, string) (IsAuthorizedResult, error)
	GetAuthorizationRecord(context.Context, string) (AuthorizationRecord, error)
	ListAuthorized(context.Context) ([]AuthorizationRecord, error)
}
