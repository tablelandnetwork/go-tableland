package impl

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/transactor"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/user"
)

// SQLStorePGX implements the SQLStore interface using pgx.
type SQLStorePGX struct {
	t *transactor.Transactor
	*user.UserStore
	*system.SystemStore
}

// New creates a new SQL store with transaction support and instantiate both the user and system stores.
func New(ctx context.Context, postgresURI string, enableTx bool) (sqlstore.SQLStore, error) {
	pool, err := pgxpool.Connect(ctx, postgresURI)
	if err != nil {
		return nil, err
	}

	var t *transactor.Transactor
	if enableTx {
		t = transactor.New(pool)
	} else {
		t = transactor.NewWithoutTransaction(pool)
	}

	userStore := user.New(t)
	systemStore, err := system.New(t)
	if err != nil {
		return nil, err
	}

	return &SQLStorePGX{t, userStore, systemStore}, nil
}

// Begin initiates a new transaction.
func (s *SQLStorePGX) Begin(ctx context.Context) error {
	return s.t.Begin(ctx)
}

// Commit commits the current ongoing transaction.
func (s *SQLStorePGX) Commit(ctx context.Context) error {
	return s.t.Commit(ctx)
}

// Rollback rollbacks the current ongoing transaction.
func (s *SQLStorePGX) Rollback(ctx context.Context) error {
	return s.t.Rollback(ctx)
}

// Close closes the connection pool.
func (s *SQLStorePGX) Close() {
	s.t.Close()
}
