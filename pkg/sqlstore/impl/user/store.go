package user

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/textileio/go-tableland/pkg/parsing"
	txnimpl "github.com/textileio/go-tableland/pkg/txn/impl"
)

// UserStore provides access to the db store.
type UserStore struct {
	pool *pgxpool.Pool
}

// New creates a new UserStore.
func New(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool}
}

// Read executes a read statement on the db.
func (db *UserStore) Read(ctx context.Context, rq parsing.SugaredReadStmt) (interface{}, error) {
	var ret interface{}
	f := func(tx pgx.Tx) error {
		dbName, err := txnimpl.GetTableNameByTableID(ctx, tx, rq.GetTableID())
		if err != nil {
			return fmt.Errorf("table name lookup for table id: %s", err)
		}
		wqName := rq.GetNamePrefix()
		if wqName != "" && dbName != wqName {
			return fmt.Errorf("table name prefix doesn't match (exp %s, got %s)", dbName, wqName)
		}

		desugared, err := rq.GetDesugaredQuery()
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

func execReadQuery(ctx context.Context, tx pgx.Tx, q string) (interface{}, error) {
	rows, err := tx.Query(ctx, q, pgx.QuerySimpleProtocol(true))
	if err != nil {
		return nil, fmt.Errorf("executing query: %s", err)
	}
	defer rows.Close()
	return rowsToJSON(rows)
}
