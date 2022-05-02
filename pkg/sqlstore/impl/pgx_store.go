package impl

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/user"
)

// SQLStorePGX implements the SQLStore interface using pgx.
type SQLStorePGX struct {
	pool *pgxpool.Pool
	*user.UserStore
	*system.SystemStore
}

// Close closes the connection pool.
func (db *SQLStorePGX) Close() {
	db.pool.Close()
}

// New creates a new pgx pool and instantiate both the user and system stores.
func New(ctx context.Context, chainID tableland.ChainID, postgresURI string) (sqlstore.SQLStore, error) {
	pool, err := pgxpool.Connect(ctx, postgresURI)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %s", err)
	}

	userStore := user.New(pool, chainID)
	systemStore, err := system.New(pool, chainID)
	if err != nil {
		return nil, fmt.Errorf("creating system store: %s", err)
	}

	return &SQLStorePGX{pool, userStore, systemStore}, nil
}
