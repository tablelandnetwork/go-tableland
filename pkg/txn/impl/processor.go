package impl

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/txn"
)

// TblTxnProcessor executes mutating actions in a Tableland database.
type TblTxnProcessor struct {
	pool *pgxpool.Pool

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
func (ab *TblTxnProcessor) OpenBatch(ctx context.Context) (txn.Batch, error) {
	<-ab.chBatch

	ops := pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadWrite,
	}
	txn, err := ab.pool.BeginTx(ctx, ops)
	if err != nil {
		ab.chBatch <- struct{}{}
		return nil, fmt.Errorf("opening postgres transaction: %s", err)
	}

	return &batch{txn: txn, p: ab}, nil
}

// Close closes the processor gracefully. It will wait for any pending
// batch to be closed, or until ctx is canceled.
func (ab *TblTxnProcessor) Close(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return errors.New("closing ctx done")
	case <-ab.chBatch:
		log.Info().Msg("txn processor closed gracefully")
		return nil
	}
}

type batch struct {
	txn pgx.Tx
	p   *TblTxnProcessor
}

// InsertTable creates a new table in Tableland:
// - Registers the table in the system-wide table registry.
// - Executes the CREATE statement.
func (b *batch) InsertTable(
	ctx context.Context,
	uuid uuid.UUID,
	controller string,
	tableType string,
	createStmt string) error {
	f := func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO system_tables ("uuid","controller","type") VALUES ($1,$2,$3);`,
			uuid, controller, sql.NullString{String: tableType, Valid: true}); err != nil {
			return fmt.Errorf("inserting new table in system-wide registry: %s", err)
		}
		if _, err := tx.Exec(ctx, createStmt); err != nil {
			return fmt.Errorf("exec CREATE statement: %s", err)
		}

		return nil
	}
	if err := b.txn.BeginFunc(ctx, f); err != nil {
		return fmt.Errorf("processing register table: %s", err)
	}
	return nil
}

func (b *batch) ExecWriteQueries(ctx context.Context, wqueries []parsing.WriteStmt) error {
	f := func(nestedTxn pgx.Tx) error {
		for _, wq := range wqueries {
			if _, err := nestedTxn.Exec(ctx, wq.GetRawQuery()); err != nil {
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
	defer func() { b.p.chBatch <- struct{}{} }()

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
