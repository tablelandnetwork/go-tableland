package sqlstore

import (
	"context"

	"github.com/textileio/go-tableland/pkg/parsing"
)

// SystemStore defines the methods for interacting with system-wide data.
type SystemStore interface {
	GetTable(context.Context, parsing.TableID) (Table, error)
	GetTablesByController(context.Context, string) ([]Table, error)
	Authorize(context.Context, string) error
	Revoke(context.Context, string) error
	IsAuthorized(context.Context, string) (IsAuthorizedResult, error)
	GetAuthorizationRecord(context.Context, string) (AuthorizationRecord, error)
	ListAuthorized(context.Context) ([]AuthorizationRecord, error)
	IncrementCreateTableCount(context.Context, string) error
	IncrementRunSQLCount(context.Context, string) error
}
