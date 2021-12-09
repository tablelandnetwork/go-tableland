package impl

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

type Postgres struct {
	ctx  context.Context
	pool *pgxpool.Pool
}

func (db *Postgres) Write(statement string) error {
	_, err := db.pool.Exec(db.ctx, statement)
	return err
}

func (db *Postgres) Read(statement string) (interface{}, error) {
	rows, err := db.pool.Query(db.ctx, statement, pgx.QuerySimpleProtocol(true))
	if err != nil {
		return []byte{}, err
	}

	defer rows.Close()
	return rowsToJSON(rows)
}

func (db *Postgres) Close() {
	db.pool.Close()
}

func NewPostgres(ctx context.Context, host, port, user, pass, name string) (sqlstore.SQLStore, error) {
	databaseUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&timezone=UTC", user, pass, host, port, name)

	pool, err := pgxpool.Connect(ctx, databaseUrl)
	if err != nil {
		return nil, err
	}

	return &Postgres{ctx, pool}, nil
}
