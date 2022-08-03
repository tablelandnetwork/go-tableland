package impl

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
	pool         *sql.DB
	parser       parsing.SQLValidator
	acl          tableland.ACL
	chBlockScope chan struct{}

	chainID          tableland.ChainID
	maxTableRowCount int
}

var _ executor.Executor = (*Executor)(nil)

// NewTxnProcessor returns a new Tableland transaction processor.
func NewExecutor(
	chainID tableland.ChainID,
	dbURI string,
	parser parsing.SQLValidator,
	maxTableRowCount int,
	acl tableland.ACL,
) (*Executor, error) {
	pool, err := otelsql.Open("sqlite3", dbURI, otelsql.WithAttributes(
		attribute.String("name", "processor"),
		attribute.Int64("chain_id", int64(chainID)),
	))
	if err != nil {
		return nil, fmt.Errorf("connecting to db: %s", err)
	}
	if err := otelsql.RegisterDBStatsMetrics(pool, otelsql.WithAttributes(
		attribute.String("name", "processor"),
		attribute.Int64("chain_id", int64(chainID)),
	)); err != nil {
		return nil, fmt.Errorf("registering dbstats: %s", err)
	}
	pool.SetMaxOpenConns(1)
	if maxTableRowCount < 0 {
		return nil, fmt.Errorf("maximum table row count is negative")
	}

	log := logger.With().
		Str("component", "txnprocessor").
		Int64("chainID", int64(chainID)).
		Logger()
	tblp := &Executor{
		log:          log,
		pool:         pool,
		parser:       parser,
		acl:          acl,
		chBlockScope: make(chan struct{}, 1),

		chainID:          chainID,
		maxTableRowCount: maxTableRowCount,
	}
	tblp.chBlockScope <- struct{}{}

	return tblp, nil
}

// OpenBatch starts a new batch of mutating actions to be executed.
func (ex *Executor) NewBlockScope(ctx context.Context, blockNum int64) (executor.BlockScope, error) {
	// TODO(jsign): panic
	<-ex.chBlockScope

	txn, err := ex.pool.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: false})
	if err != nil {
		return nil, fmt.Errorf("opening db transaction: %s", err)
	}

	scopeVars := scopeVars{ChainID: ex.chainID, MaxTableRowCount: ex.maxTableRowCount}
	bs := newBlockScope(txn, scopeVars, ex.parser, ex.acl, blockNum, func() { ex.chBlockScope <- struct{}{} })

	return bs, nil
}

// Close closes the processor gracefully. It will wait for any pending
// batch to be closed, or until ctx is canceled.
func (ex *Executor) Close(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return errors.New("closing ctx done")
	case <-ex.chBlockScope:
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
