package impl

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/XSAM/otelsql"
	"github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor"
	"github.com/textileio/go-tableland/pkg/parsing"
	"go.opentelemetry.io/otel/attribute"
)

// Executor executes chain events.
type Executor struct {
	log          zerolog.Logger
	db           *sql.DB
	parser       parsing.SQLValidator
	acl          tableland.ACL
	chBlockScope chan struct{}

	chainID          tableland.ChainID
	maxTableRowCount int

	closeOnce sync.Once
	closed    chan struct{}
}

var _ executor.Executor = (*Executor)(nil)

// NewExecutor returns a new Executor.
func NewExecutor(
	chainID tableland.ChainID,
	dbURI string,
	parser parsing.SQLValidator,
	maxTableRowCount int,
	acl tableland.ACL,
) (*Executor, error) {
	db, err := otelsql.Open("sqlite3", dbURI, otelsql.WithAttributes(
		attribute.String("name", "processor"),
		attribute.Int64("chain_id", int64(chainID)),
	))
	if err != nil {
		return nil, fmt.Errorf("connecting to db: %s", err)
	}
	db.SetMaxIdleConns(0)
	db.SetMaxOpenConns(1)
	if err := otelsql.RegisterDBStatsMetrics(db, otelsql.WithAttributes(
		attribute.String("name", "processor"),
		attribute.Int64("chain_id", int64(chainID)),
	)); err != nil {
		return nil, fmt.Errorf("registering dbstats: %s", err)
	}
	if maxTableRowCount < 0 {
		return nil, fmt.Errorf("maximum table row count is negative")
	}

	log := logger.With().
		Str("component", "executor").
		Int64("chain_id", int64(chainID)).
		Logger()
	tblp := &Executor{
		log:          log,
		db:           db,
		parser:       parser,
		acl:          acl,
		chBlockScope: make(chan struct{}, 1),

		chainID:          chainID,
		maxTableRowCount: maxTableRowCount,

		closed: make(chan struct{}),
	}
	tblp.chBlockScope <- struct{}{}

	return tblp, nil
}

// NewBlockScope starts a block scope to execute EVM transactions with events.
func (ex *Executor) NewBlockScope(ctx context.Context, newBlockNum int64) (executor.BlockScope, error) {
	select {
	case <-ex.chBlockScope:
	case <-ex.closed:
		return nil, fmt.Errorf("executor is closed")
	default:
		panic("parallel block scope detected, this must never happen")
	}

	txn, err := ex.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: false})
	if err != nil {
		return nil, fmt.Errorf("opening db transaction: %s", err)
	}

	// Check that the last processed height is strictly lower.
	lastBlockNum, err := ex.getLastExecutedBlockNumber(ctx, txn)
	if err != nil {
		return nil, fmt.Errorf("get last processed height: %s", err)
	}
	if lastBlockNum >= newBlockNum {
		return nil, fmt.Errorf("latest executed block %d isn't smaller than new block %d", lastBlockNum, newBlockNum)
	}

	scopeVars := scopeVars{ChainID: ex.chainID, MaxTableRowCount: ex.maxTableRowCount}
	bs := newBlockScope(txn, scopeVars, ex.parser, ex.acl, newBlockNum, func() { ex.chBlockScope <- struct{}{} })

	return bs, nil
}

// GetLastExecutedBlockNumber returns the last block number that was successfully executed.
func (ex *Executor) GetLastExecutedBlockNumber(ctx context.Context) (int64, error) {
	txn, err := ex.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("opening txn: %s", err)
	}
	defer func() {
		_ = txn.Rollback()
	}()
	blockNumber, err := ex.getLastExecutedBlockNumber(ctx, txn)
	if err != nil {
		return 0, fmt.Errorf("get last processed block number: %s", err)
	}
	return blockNumber, nil
}

func (ex *Executor) getLastExecutedBlockNumber(ctx context.Context, txn *sql.Tx) (int64, error) {
	r := txn.QueryRowContext(
		ctx,
		"SELECT block_number FROM system_txn_processor WHERE chain_id=?1 LIMIT 1",
		ex.chainID)
	var blockNumber int64
	if err := r.Scan(&blockNumber); err != nil {
		if err == sql.ErrNoRows {
			return -1, nil
		}
		return 0, fmt.Errorf("get last block number query: %s", err)
	}
	return blockNumber, nil
}

// Close closes the processor gracefully. It will wait for any pending
// batch to be closed, or until ctx is canceled.
func (ex *Executor) Close(ctx context.Context) error {
	ex.closeOnce.Do(func() { close(ex.closed) })
	select {
	case <-ctx.Done():
		if err := ex.db.Close(); err != nil {
			ex.log.Error().Err(err).Msg("forced close of database connection")
		}
		return errors.New("closing ctx done")
	case <-ex.chBlockScope:
		if err := ex.db.Close(); err != nil {
			ex.log.Error().Err(err).Msg("closing database connection")
		}
		ex.log.Info().Msg("executor closed gracefully")
		return nil
	}
}

// isErrCausedByQuery detects if the query execution failed because of possibly expected
// bad queries from users. If that's the case the call might want to accept the failure
// as an expected event in the flow.
func isErrCausedByQuery(err error) (string, bool) {
	// This array contains all the sqlite errors that should be query related.
	// e.g: inserting a column with the wrong type, some function call failing, etc.
	// All these errors must be errors that will always happen if the query is retried.
	// (e.g: a timeout error isn't the querys fault, but an infrastructure problem)
	//
	// Each error in sqlite3 has an "Error Code" and an "Extended error code".
	// e.g: a FK violation has "Error Code" 19 (ErrConstraint) and
	// "Extended error code" 787 (SQLITE_CONSTRAINT_FOREIGNKEY).
	// The complete list of extended errors is found in: https://www.sqlite.org/rescode.html
	// In this logic if we use "Error Code", with some few cases, we can detect a wide range of errors without
	// being so exhaustive dealing with "Extended error codes".
	//
	// sqlite3ExecutionErrors is probably missing values, but we'll keep discovering and adding them.
	sqlite3ExecutionErrors := []sqlite3.ErrNo{
		sqlite3.ErrError,      /* SQL error or missing database */
		sqlite3.ErrConstraint, /* Abort due to constraint violation */
		sqlite3.ErrTooBig,     /* String or BLOB exceeds size limit */
		sqlite3.ErrMismatch,   /* Data type mismatch */
	}
	var sqlErr sqlite3.Error
	if errors.As(err, &sqlErr) {
		for _, ee := range sqlite3ExecutionErrors {
			if sqlErr.Code == ee {
				return sqlErr.Error(), true
			}
		}
	}
	return "", false
}
