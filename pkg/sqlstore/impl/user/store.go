package user

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/XSAM/otelsql"
	_ "github.com/mattn/go-sqlite3" // sqlite3 driver
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"go.opentelemetry.io/otel/attribute"
)

var log = logger.With().Str("component", "userstore").Logger()

// UserStore provides access to the db store.
type UserStore struct {
	pool *sql.DB
}

// New creates a new UserStore.
func New(dbURI string) (*UserStore, error) {
	pool, err := otelsql.Open("sqlite3", dbURI, otelsql.WithAttributes(
		attribute.String("name", "userstore"),
	))
	if err != nil {
		return nil, fmt.Errorf("connecting to db: %s", err)
	}
	if err := otelsql.RegisterDBStatsMetrics(pool, otelsql.WithAttributes(
		attribute.String("name", "userstore"),
	)); err != nil {
		return nil, fmt.Errorf("registering dbstats: %s", err)
	}
	return &UserStore{
		pool: pool,
	}, nil
}

// Read executes a read statement on the db.
func (db *UserStore) Read(ctx context.Context, rq parsing.ReadStmt) (interface{}, error) {
	query, err := rq.GetQuery()
	if err != nil {
		return nil, fmt.Errorf("get query: %s", err)
	}
	ret, err := execReadQuery(ctx, db.pool, query)
	if err != nil {
		return nil, fmt.Errorf("parsing result to json: %s", err)
	}
	return ret, nil
}

// Close closes the store.
func (db *UserStore) Close() error {
	if err := db.pool.Close(); err != nil {
		return fmt.Errorf("closing db: %s", err)
	}
	return nil
}

func execReadQuery(ctx context.Context, tx *sql.DB, q string) (*sqlstore.UserRows, error) {
	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("executing query: %s", err)
	}
	defer func() {
		if err = rows.Close(); err != nil {
			log.Warn().Err(err).Msg("closing rows")
		}
	}()
	return rowsToJSON(rows)
}
