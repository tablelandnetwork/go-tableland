package system

import (
	"context"
	"math/big"

	"github.com/textileio/go-tableland/pkg/sqlstore"
)

// SystemService defines what system operations can be done.
type SystemService interface {
	GetTableMetadata(context.Context, *big.Int) (sqlstore.TableMetadata, error)
	GetTablesByController(context.Context, string) ([]sqlstore.Table, error)
	Authorize(context.Context, string) error
	Revoke(context.Context, string) error
	IsAuthorized(context.Context, string) (sqlstore.IsAuthorizedResult, error)
	GetAuthorizationRecord(context.Context, string) (sqlstore.AuthorizationRecord, error)
	ListAuthorized(context.Context) ([]sqlstore.AuthorizationRecord, error)
}
