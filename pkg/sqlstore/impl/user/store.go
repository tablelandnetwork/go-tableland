package user

import (
	"context"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type UserStore struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool}
}

func (db *UserStore) Write(ctx context.Context, statement string) error {
	_, err := db.pool.Exec(ctx, statement)
	return err
}

func (db *UserStore) Read(ctx context.Context, statement string) (interface{}, error) {
	rows, err := db.pool.Query(ctx, statement, pgx.QuerySimpleProtocol(true))
	if err != nil {
		return []byte{}, err
	}

	defer rows.Close()
	return rowsToJSON(rows)
}
