package sqlstore

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v4"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/nonce"
)

// SystemStore defines the methods for interacting with system-wide data.
type SystemStore interface {
	GetTable(context.Context, tableland.TableID) (Table, error)
	GetTablesByController(context.Context, string) ([]Table, error)
	Authorize(context.Context, string) error
	Revoke(context.Context, string) error
	IsAuthorized(context.Context, string) (IsAuthorizedResult, error)
	GetAuthorizationRecord(context.Context, string) (AuthorizationRecord, error)
	ListAuthorized(context.Context) ([]AuthorizationRecord, error)
	IncrementCreateTableCount(context.Context, string) error
	IncrementRunSQLCount(context.Context, string) error
	GetACLOnTableByController(context.Context, tableland.TableID, string) (SystemACL, error)
	GetNonce(context.Context, string, common.Address) (nonce.Nonce, error)
	UpsertNonce(context.Context, string, common.Address, int64) error
	ListPendingTx(context.Context, string, common.Address) ([]nonce.PendingTx, error)
	InsertPendingTx(context.Context, string, common.Address, int64, common.Hash) error
	DeletePendingTxByHash(context.Context, common.Hash) error
	WithTx(tx pgx.Tx) SystemStore
	Begin(context.Context) (pgx.Tx, error)
	GetReceipt(context.Context, int64, string) (eventprocessor.Receipt, bool, error)
}
