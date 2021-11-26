package impl

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

type Postgres struct {
	ctx  context.Context
	conn *pgx.Conn
}

func (db *Postgres) Query(statement string) error {
	_, err := db.conn.Query(db.ctx, statement)
	return err
}

func NewPostgres(ctx context.Context, host, port, user, pass, name string) (sqlstore.SQLStore, error) {
	databaseUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&timezone=UTC", user, pass, host, port, name)

	conn, err := pgx.Connect(ctx, databaseUrl)
	if err != nil {
		return nil, err
	}

	return &Postgres{ctx, conn}, nil
}
