package impl

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

type Postgres struct {
	ctx  context.Context
	pool *pgxpool.Pool
}

func (db *Postgres) Query(statement string) error {
	_, err := db.pool.Exec(db.ctx, statement)
	return err
}

func NewPostgres(ctx context.Context, host, port, user, pass, name string) (sqlstore.SQLStore, error) {
	databaseUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&timezone=UTC", user, pass, host, port, name)

	pool, err := pgxpool.Connect(ctx, databaseUrl)
	if err != nil {
		return nil, err
	}

	return &Postgres{ctx, pool}, nil
}
