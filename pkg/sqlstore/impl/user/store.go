package user

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

// UserStore provides access to the db store.
type UserStore struct {
	pool *sql.DB
}

// New creates a new UserStore.
func New(sqliteURI string) (*UserStore, error) {
	pool, err := sql.Open("sqlite3", sqliteURI)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %s", err)
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
	db.pool.Close()
	return nil
}

func execReadQuery(ctx context.Context, tx *sql.DB, q string) (*sqlstore.UserRows, error) {
	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("executing query: %s", err)
	}
	defer rows.Close()
	return rowsToJSON(rows)
}
