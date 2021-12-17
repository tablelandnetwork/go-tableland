package user

import (
	"context"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// UserStore provides access to the db store.
type UserStore struct {
	pool *pgxpool.Pool
}

// New creates a new UserStore.
func New(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool}
}

// Write executes a write statement on the db.
func (db *UserStore) Write(ctx context.Context, statement string) error {
	_, err := db.pool.Exec(ctx, statement)
	return err
}

// Read executes a read statement on the db.
func (db *UserStore) Read(ctx context.Context, statement string) (interface{}, error) {
	rows, err := db.pool.Query(ctx, statement, pgx.QuerySimpleProtocol(true))
	if err != nil {
		return []byte{}, err
	}

	defer rows.Close()
	return rowsToJSON(rows)
}
