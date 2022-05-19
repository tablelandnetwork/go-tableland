package user

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/textileio/go-tableland/pkg/parsing"
)

// UserStore provides access to the db store.
type UserStore struct {
	pool *pgxpool.Pool
}

// New creates a new UserStore.
func New(postgresURI string) (*UserStore, error) {
	ctx, cls := context.WithTimeout(context.Background(), time.Second*15)
	defer cls()
	pool, err := pgxpool.Connect(ctx, postgresURI)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %s", err)
	}
	return &UserStore{
		pool: pool,
	}, nil
}

// Read executes a read statement on the db.
func (db *UserStore) Read(ctx context.Context, rq parsing.ReadStmt) (interface{}, error) {
	var ret interface{}
	f := func(tx pgx.Tx) error {
		desugared, err := rq.GetQuery()
		if err != nil {
			return fmt.Errorf("get desugared query: %s", err)
		}
		ret, err = execReadQuery(ctx, tx, desugared)
		if err != nil {
			return fmt.Errorf("parsing result to json: %s", err)
		}
		return nil
	}
	if err := db.pool.BeginFunc(ctx, f); err != nil {
		return nil, fmt.Errorf("running nested txn: %s", err)
	}
	return ret, nil
}

func (db *UserStore) Close() error {
	db.pool.Close()
	return nil
}

func execReadQuery(ctx context.Context, tx pgx.Tx, q string) (interface{}, error) {
	rows, err := tx.Query(ctx, q, pgx.QuerySimpleProtocol(true))
	if err != nil {
		return nil, fmt.Errorf("executing query: %s", err)
	}
	defer rows.Close()
	return rowsToJSON(rows)
}
