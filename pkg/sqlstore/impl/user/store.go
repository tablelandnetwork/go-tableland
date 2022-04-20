package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
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
		wqName := rq.GetNamePrefix()
		if wqName != "" {
			dbName, _, err := txnimpl.GetTableNameAndRowCountByTableID(ctx, tx, rq.GetTableID())
			if err != nil {
				return fmt.Errorf("table name lookup for table id: %s", err)
			}
			if dbName != wqName {
				return fmt.Errorf("table name prefix doesn't match (exp %s, got %s)", dbName, wqName)
			}
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

// GetReceipt returns a event receipt by transaction hash.
func (db *UserStore) GetReceipt(
	ctx context.Context,
	chainID int64,
	txnHash string) (eventprocessor.Receipt, bool, error) {
	var dbError sql.NullString
	var dbTableID pgtype.Numeric
	row := db.pool.QueryRow(
		ctx,
		"SELECT error, table_id FROM system_txn_receipts WHERE chain_id=$1 AND txn_hash=$2",
		chainID, txnHash)
	err := row.Scan(&dbError, &dbTableID)
	if err == pgx.ErrNoRows {
		return eventprocessor.Receipt{}, false, nil
	}
	if err != nil {
		return eventprocessor.Receipt{}, false, fmt.Errorf("get txn receipt: %s", err)
	}

	receipt := eventprocessor.Receipt{
		ChainID: chainID,
		TxnHash: txnHash,
	}
	if dbTableID.Status == pgtype.Present {
		br := &big.Rat{}
		if err := dbTableID.AssignTo(br); err != nil {
			return eventprocessor.Receipt{}, false, fmt.Errorf("parsing numeric to bigrat: %s", err)
		}
		if !br.IsInt() {
			return eventprocessor.Receipt{}, false, errors.New("parsed numeric isn't an integer")
		}
		id, err := tableland.NewTableID(br.Num().String())
		if err != nil {
			return eventprocessor.Receipt{}, false, fmt.Errorf("parsing id to string: %s", err)
		}
		receipt.TableID = &id
	}
	if dbError.Valid {
		receipt.Error = &dbError.String
	}

	return receipt, true, nil
}

func execReadQuery(ctx context.Context, tx pgx.Tx, q string) (interface{}, error) {
	rows, err := tx.Query(ctx, q, pgx.QuerySimpleProtocol(true))
	if err != nil {
		return nil, fmt.Errorf("executing query: %s", err)
	}
	defer rows.Close()
	return rowsToJSON(rows)
}
