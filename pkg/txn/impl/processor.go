package impl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/txn"
)

// TblTxnProcessor executes mutating actions in a Tableland database.
type TblTxnProcessor struct {
	pool    *pgxpool.Pool
	chBatch chan struct{}
}

var _ txn.TxnProcessor = (*TblTxnProcessor)(nil)

// NewTxnProcessor returns a new Tableland transaction processor.
func NewTxnProcessor(postgresURI string) (*TblTxnProcessor, error) {
	ctx, cls := context.WithTimeout(context.Background(), time.Second*10)
	defer cls()
	pool, err := pgxpool.Connect(ctx, postgresURI)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %s", err)
	}
	tblp := &TblTxnProcessor{
		pool:    pool,
		chBatch: make(chan struct{}, 1),
	}
	tblp.chBatch <- struct{}{}

	return tblp, nil
}

// OpenBatch starts a new batch of mutating actions to be executed.
// If a batch is already open, it will wait until is finishes. This is on purpose
// since mutating actions should be processed serially.
func (tp *TblTxnProcessor) OpenBatch(ctx context.Context) (txn.Batch, error) {
	<-tp.chBatch

	ops := pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadWrite,
	}
	txn, err := tp.pool.BeginTx(ctx, ops)
	if err != nil {
		tp.chBatch <- struct{}{}
		return nil, fmt.Errorf("opening postgres transaction: %s", err)
	}

	return &batch{txn: txn, tp: tp}, nil
}

// Close closes the processor gracefully. It will wait for any pending
// batch to be closed, or until ctx is canceled.
func (tp *TblTxnProcessor) Close(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return errors.New("closing ctx done")
	case <-tp.chBatch:
		log.Info().Msg("txn processor closed gracefully")
		return nil
	}
}

type batch struct {
	txn pgx.Tx
	tp  *TblTxnProcessor
}

// InsertTable creates a new table in Tableland:
// - Registers the table in the system-wide table registry.
// - Executes the CREATE statement.
func (b *batch) InsertTable(
	ctx context.Context,
	id tableland.TableID,
	controller string,
	description string,
	createStmt parsing.CreateStmt) error {
	f := func(tx pgx.Tx) error {
		dbID := pgtype.Numeric{}
		if err := dbID.Set(id.String()); err != nil {
			return fmt.Errorf("parsing table id to numeric: %s", err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO system_tables ("id","controller","name", "structure","description") 
			 VALUES ($1,$2,$3,$4,$5);`,
			dbID,
			controller,
			createStmt.GetNamePrefix(),
			createStmt.GetStructureHash(),
			description); err != nil {
			return fmt.Errorf("inserting new table in system-wide registry: %s", err)
		}
		query, err := createStmt.GetRawQueryForTableID(id)
		if err != nil {
			return fmt.Errorf("get query for table id: %s", err)
		}
		if _, err := tx.Exec(ctx, query); err != nil {
			return fmt.Errorf("exec CREATE statement: %s", err)
		}

		return nil
	}
	if err := b.txn.BeginFunc(ctx, f); err != nil {
		return fmt.Errorf("processing register table: %s", err)
	}
	return nil
}

func (b *batch) ExecWriteQueries(ctx context.Context, wqueries []parsing.SugaredWriteStmt) error {
	f := func(tx pgx.Tx) error {
		if len(wqueries) == 0 {
			log.Warn().Msg("no write-queries to execute in a batch")
			return nil
		}
		dbName, err := GetTableNameByTableID(ctx, tx, wqueries[0].GetTableID())
		if err != nil {
			return fmt.Errorf("table name lookup for table id: %s", err)
		}
		for _, wq := range wqueries {
			wqName := wq.GetNamePrefix()
			if wqName != "" && dbName != wqName {
				return fmt.Errorf("table name prefix doesn't match (exp %s, got %s)", dbName, wqName)
			}
			desugared, err := wq.GetDesugaredQuery()
			if err != nil {
				return fmt.Errorf("get desugared query: %s", err)
			}
			if _, err := tx.Exec(ctx, desugared); err != nil {
				return fmt.Errorf("exec query: %s", err)
			}
		}

		return nil
	}
	if err := b.txn.BeginFunc(ctx, f); err != nil {
		return fmt.Errorf("running nested txn: %s", err)
	}

	return nil
}

// Close closes gracefully the batch. Clients should *always* `defer Close()` when
// opening batches.
func (b *batch) Close(ctx context.Context) error {
	defer func() { b.tp.chBatch <- struct{}{} }()

	// Calling rollback is always safe:
	// - If Commit() wasn't called, the result is a rollback.
	// - If Commit() was called, pgx.Txn guarantees is a noop.
	if err := b.txn.Rollback(ctx); err != nil {
		if err != pgx.ErrTxClosed {
			return fmt.Errorf("closing batch: %s", err)
		}
	}

	return nil
}

func (b *batch) Commit(ctx context.Context) error {
	if err := b.txn.Commit(ctx); err != nil {
		return fmt.Errorf("commit txn: %s", err)
	}
	return nil
}

func GetTableNameByTableID(ctx context.Context, tx pgx.Tx, id tableland.TableID) (string, error) {
	dbID := pgtype.Numeric{}
	if err := dbID.Set(id.String()); err != nil {
		return "", fmt.Errorf("parsing table id to numeric: %s", err)
	}
	r := tx.QueryRow(ctx, `SELECT name FROM system_tables where id=$1`, dbID)
	var dbName string
	err := r.Scan(&dbName)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("the table id doesn't exist")
	}
	if err != nil {
		return "", fmt.Errorf("table name lookup: %s", err)
	}
	return dbName, nil
}
