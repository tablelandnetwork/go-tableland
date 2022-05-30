package impl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/txn"
)

// TblTxnProcessor executes mutating actions in a Tableland database.
type TblTxnProcessor struct {
	log     zerolog.Logger
	chainID tableland.ChainID
	pool    *pgxpool.Pool
	chBatch chan struct{}

	maxTableRowCount int
	acl              tableland.ACL
}

var _ txn.TxnProcessor = (*TblTxnProcessor)(nil)

// NewTxnProcessor returns a new Tableland transaction processor.
func NewTxnProcessor(
	chainID tableland.ChainID,
	postgresURI string,
	maxTableRowCount int,
	acl tableland.ACL) (*TblTxnProcessor, error) {
	ctx, cls := context.WithTimeout(context.Background(), time.Second*10)
	defer cls()
	pool, err := pgxpool.Connect(ctx, postgresURI)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %s", err)
	}
	if maxTableRowCount < 0 {
		return nil, fmt.Errorf("maximum table row count is negative")
	}

	log := logger.With().
		Str("component", "txnprocessor").
		Int64("chainID", int64(chainID)).
		Logger()
	tblp := &TblTxnProcessor{
		log:     log,
		chainID: chainID,
		pool:    pool,
		chBatch: make(chan struct{}, 1),

		maxTableRowCount: maxTableRowCount,
		acl:              acl,
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
		tp.log.Info().Msg("txn processor closed gracefully")
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
// - Add default privileges in the system_acl table.
func (b *batch) InsertTable(
	ctx context.Context,
	id tableland.TableID,
	controller string,
	createStmt parsing.CreateStmt) error {
	f := func(tx pgx.Tx) error {
		dbID := pgtype.Numeric{}
		if err := dbID.Set(id.String()); err != nil {
			return fmt.Errorf("parsing table id to numeric: %s", err)
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO registry ("chain_id", "id","controller","prefix", "structure") 
			 VALUES ($1,$2,$3,$4, $5);`,
			b.tp.chainID,
			dbID,
			controller,
			createStmt.GetPrefix(),
			createStmt.GetStructureHash()); err != nil {
			return fmt.Errorf("inserting new table in system-wide registry: %s", err)
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO system_acl ("chain_id","table_id","controller","privileges") 
			 VALUES ($1,$2,$3,$4);`,
			b.tp.chainID,
			dbID,
			controller,
			[]string{"a", "w", "d"}, // the abbreviations for PrivInsert, PrivUpdate and PrivDelete
		); err != nil {
			return fmt.Errorf("inserting new entry into system acl: %s", err)
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

func (b *batch) ExecWriteQueries(
	ctx context.Context,
	controller common.Address,
	mqueries []parsing.MutatingStmt,
	isOwner bool,
	policy tableland.Policy) error {
	f := func(tx pgx.Tx) error {
		if len(mqueries) == 0 {
			b.tp.log.Warn().Msg("no mutating-queries to execute in a batch")
			return nil
		}

		dbTableName := mqueries[0].GetDBTableName()
		tablePrefix, beforeRowCount, err := GetTablePrefixAndRowCountByTableID(
			ctx, tx, b.tp.chainID, mqueries[0].GetTableID(), dbTableName)
		if err != nil {
			return &txn.ErrQueryExecution{
				Code: "TABLE_LOOKUP",
				Msg:  fmt.Sprintf("table prefix lookup for table id: %s", err),
			}
		}

		for _, mq := range mqueries {
			mqPrefix := mq.GetPrefix()
			if mqPrefix != "" && tablePrefix != mqPrefix {
				return &txn.ErrQueryExecution{
					Code: "TABLE_PREFIX",
					Msg:  fmt.Sprintf("table prefix doesn't match (exp %s, got %s)", tablePrefix, mqPrefix),
				}
			}

			switch stmt := mq.(type) {
			case parsing.GrantStmt:
				err := b.executeGrantStmt(ctx, tx, stmt, isOwner)
				if err != nil {
					return fmt.Errorf("executing grant stmt: %w", err)
				}
			case parsing.WriteStmt:
				if err := b.executeWriteStmt(ctx, tx, stmt, controller, policy, beforeRowCount); err != nil {
					return fmt.Errorf("executing write stmt: %w", err)
				}
			default:
				return fmt.Errorf("unknown stmt type")
			}
		}

		return nil
	}
	if err := b.txn.BeginFunc(ctx, f); err != nil {
		return fmt.Errorf("running nested txn: %w", err)
	}

	return nil
}

// SetController sets and unsets the controller of a table.
func (b *batch) SetController(
	ctx context.Context,
	id tableland.TableID,
	controller common.Address) error {
	f := func(tx pgx.Tx) error {
		dbID := pgtype.Numeric{}
		if err := dbID.Set(id.String()); err != nil {
			return fmt.Errorf("parsing table id to numeric: %s", err)
		}

		if controller == common.HexToAddress("0x0") {
			if _, err := tx.Exec(ctx,
				`DELETE FROM system_controller WHERE chain_id = $1 AND table_id = $2;`,
				b.tp.chainID,
				dbID,
			); err != nil {
				if code, ok := isErrCausedByQuery(err); ok {
					return &txn.ErrQueryExecution{
						Code: "POSTGRES_" + code,
						Msg:  err.Error(),
					}
				}
				return fmt.Errorf("deleting entry from system controller: %s", err)
			}
		} else {
			if _, err := tx.Exec(ctx,
				`INSERT INTO system_controller ("chain_id", "table_id", "controller") 
				VALUES ($1, $2, $3)
				ON CONFLICT ("chain_id", "table_id")
				DO UPDATE set controller = $3;`,
				b.tp.chainID,
				dbID,
				controller.Hex(),
			); err != nil {
				if code, ok := isErrCausedByQuery(err); ok {
					return &txn.ErrQueryExecution{
						Code: "POSTGRES_" + code,
						Msg:  err.Error(),
					}
				}
				return fmt.Errorf("inserting new entry into system controller: %s", err)
			}
		}

		return nil
	}
	if err := b.txn.BeginFunc(ctx, f); err != nil {
		return fmt.Errorf("processing set controller: %s", err)
	}
	return nil
}

// GrantPrivileges gives privileges to an address on a table.
func (b *batch) GrantPrivileges(
	ctx context.Context,
	id tableland.TableID,
	addr common.Address,
	privileges tableland.Privileges) error {
	f := func(tx pgx.Tx) error {
		if err := b.executeGrantPrivilegesTx(ctx, tx, id, addr, privileges); err != nil {
			return fmt.Errorf("executing grant privileges tx: %w", err)
		}
		return nil
	}
	if err := b.txn.BeginFunc(ctx, f); err != nil {
		return fmt.Errorf("processing grant privileges: %s", err)
	}
	return nil
}

// RevokePrivileges revokes privileges from an address on a table.
func (b *batch) RevokePrivileges(
	ctx context.Context,
	id tableland.TableID,
	addr common.Address,
	privileges tableland.Privileges) error {
	f := func(tx pgx.Tx) error {
		if err := b.executeRevokePrivilegesTx(ctx, tx, id, addr, privileges); err != nil {
			return fmt.Errorf("executing revoke privileges tx: %w", err)
		}
		return nil
	}
	if err := b.txn.BeginFunc(ctx, f); err != nil {
		return fmt.Errorf("processing revoke privileges: %s", err)
	}
	return nil
}

// isErrCausedByQuery detects if the query execution failed because of possibly expected
// bad queries from users. If that's the case the call might want to accept the failure
// as an expected event in the flow.
func isErrCausedByQuery(err error) (string, bool) {
	// This array contains all the postgres errors that should be query related.
	// e.g: inserting a column with the wrong type, some function call failing, etc.
	// All these errors must be errors that will always happen if the query is retried.
	// (e.g: a timeout error isn't the querys fault, but an infrastructure problem)
	// The complete list of errors is found in: https://www.postgresql.org/docs/current/errcodes-appendix.html
	// pgExecutionErrors is probably missing values, but we'll keep discovering and adding them.
	pgExecutionErrors := []string{
		// Class 22 — Data Exception
		"22P02", // invalid_text_representation (Caused by a query trying to insert a wrong column type.)
		"22021", // invalid byte sequence encoding

		// Class 23 — Integrity Constraint Violation
		"23502", // not_null_violation
		"23505", // unique_violation
		"23514", // check_violation

		// Class 0L - Invalid Grantor
		"0L000", //	invalid_grantor
		"0LP01", //	invalid_grant_operation

		// Class 0P — Invalid Role Specification
		"0P000", //	invalid_role_specification

		// Class 42 - Syntax Error or Access Rule Violation
		"42703", // undefined_column
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		for _, ee := range pgExecutionErrors {
			if pgErr.Code == ee {
				return pgErr.Code, true
			}
		}
	}
	return "", false
}

func (b *batch) GetLastProcessedHeight(ctx context.Context) (int64, error) {
	var blockNumber int64
	f := func(tx pgx.Tx) error {
		r := tx.QueryRow(ctx, "SELECT block_number FROM system_txn_processor WHERE chain_id=$1 LIMIT 1", b.tp.chainID)
		if err := r.Scan(&blockNumber); err != nil {
			if err == pgx.ErrNoRows {
				blockNumber = 0
				return nil
			}
			return fmt.Errorf("get last block number query: %s", err)
		}
		return nil
	}
	if err := b.txn.BeginFunc(ctx, f); err != nil {
		return 0, fmt.Errorf("processing register table: %s", err)
	}
	return blockNumber, nil
}

func (b *batch) SetLastProcessedHeight(ctx context.Context, height int64) error {
	f := func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, "UPDATE system_txn_processor set block_number=$1 WHERE chain_id=$2", height, b.tp.chainID)
		if err != nil {
			return fmt.Errorf("update last processed block number: %s", err)
		}
		if tag.RowsAffected() != 1 {
			_, err := tx.Exec(ctx,
				"INSERT INTO system_txn_processor (block_number, chain_id) VALUES ($1, $2)",
				height,
				b.tp.chainID,
			)
			if err != nil {
				return fmt.Errorf("inserting first processed height: %s", err)
			}
		}
		return nil
	}
	if err := b.txn.BeginFunc(ctx, f); err != nil {
		return fmt.Errorf("set last processed height: %s", err)
	}
	return nil
}

func (b *batch) SaveTxnReceipts(ctx context.Context, rs []eventprocessor.Receipt) error {
	f := func(tx pgx.Tx) error {
		for _, r := range rs {
			dbID := pgtype.Numeric{Status: pgtype.Null}
			if r.TableID != nil {
				if err := dbID.Set(r.TableID.String()); err != nil {
					return fmt.Errorf("parsing table id to numeric: %s", err)
				}
			}
			if r.Error != nil {
				*r.Error = strings.ToValidUTF8(*r.Error, "")
			}
			if _, err := tx.Exec(
				ctx,
				`INSERT INTO system_txn_receipts (chain_id, txn_hash, error, table_id, block_number) 
				 VALUES ($1, $2, $3, $4, $5)`,
				r.ChainID, r.TxnHash, r.Error, dbID, r.BlockNumber); err != nil {
				return fmt.Errorf("insert txn receipt: %s", err)
			}
		}
		return nil
	}
	if err := b.txn.BeginFunc(ctx, f); err != nil {
		return fmt.Errorf("saving txn receipt: %s", err)
	}
	return nil
}

func (b *batch) TxnReceiptExists(ctx context.Context, txnHash common.Hash) (bool, error) {
	var exists bool
	f := func(tx pgx.Tx) error {
		r := tx.QueryRow(
			ctx,
			`SELECT 1 from system_txn_receipts WHERE chain_id=$1 and txn_hash=$2`,
			b.tp.chainID, txnHash.Hex())
		var dummy int
		err := r.Scan(&dummy)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return fmt.Errorf("get txn receipt: %s", err)
		}
		exists = true
		return nil
	}
	if err := b.txn.BeginFunc(ctx, f); err != nil {
		return false, fmt.Errorf("txn receipt lookup: %s", err)
	}
	return exists, nil
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

// GetTablePrefixAndRowCountByTableID returns the table prefix and current row count for a TableID
// within the provided transaction.
func GetTablePrefixAndRowCountByTableID(
	ctx context.Context,
	tx pgx.Tx,
	chainID tableland.ChainID,
	tableID tableland.TableID,
	dbTableName string) (string, int, error) {
	dbID := pgtype.Numeric{}
	if err := dbID.Set(tableID.String()); err != nil {
		return "", 0, fmt.Errorf("parsing table id to numeric: %s", err)
	}

	q := fmt.Sprintf(
		"SELECT (SELECT prefix FROM registry where chain_id=$1 AND id=$2), (SELECT count(*) FROM %s)", dbTableName)
	r := tx.QueryRow(ctx, q, chainID, dbID)
	var tablePrefix string
	var rowCount int
	err := r.Scan(&tablePrefix, &rowCount)
	if err == pgx.ErrNoRows {
		return "", 0, fmt.Errorf("the table id doesn't exist")
	}
	if err != nil {
		return "", 0, fmt.Errorf("table prefix lookup: %s", err)
	}
	return tablePrefix, rowCount, nil
}

// getController gets the controller for a given table.
func getController(
	ctx context.Context,
	tx pgx.Tx,
	chainID tableland.ChainID,
	tableID tableland.TableID) (string, error) {
	dbID := pgtype.Numeric{}
	if err := dbID.Set(tableID.String()); err != nil {
		return "", &txn.ErrQueryExecution{
			Code: "CONTROLLER_TABLE_ID",
			Msg:  fmt.Sprintf("parsing table id to numeric: %s", err),
		}
	}

	q := "SELECT controller FROM system_controller where chain_id=$1 AND table_id=$2"
	r := tx.QueryRow(ctx, q, chainID, dbID)
	var controller string
	err := r.Scan(&controller)
	if err == pgx.ErrNoRows {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("controller lookup: %s", err)
	}
	return controller, nil
}

func (b *batch) executeGrantStmt(
	ctx context.Context,
	tx pgx.Tx,
	gs parsing.GrantStmt,
	isOwner bool) error {
	tableID := gs.GetTableID()

	dbID := pgtype.Numeric{}
	if err := dbID.Set(tableID.String()); err != nil {
		return &txn.ErrQueryExecution{
			Code: "ACL_TABLE_ID",
			Msg:  fmt.Sprintf("parsing table id to numeric: %s", err),
		}
	}

	if !isOwner {
		return &txn.ErrQueryExecution{
			Code: "ACL_NOT_OWNER",
			Msg:  "non owner cannot execute grant stmt",
		}
	}

	for _, role := range gs.GetRoles() {
		switch gs.Operation() {
		case tableland.OpGrant:
			if err := b.executeGrantPrivilegesTx(ctx, tx, gs.GetTableID(), role, gs.GetPrivileges()); err != nil {
				return fmt.Errorf("executing grant privileges tx: %w", err)
			}
		case tableland.OpRevoke:
			if err := b.executeRevokePrivilegesTx(ctx, tx, gs.GetTableID(), role, gs.GetPrivileges()); err != nil {
				return fmt.Errorf("executing revoke privileges tx: %w", err)
			}
		default:
			return &txn.ErrQueryExecution{
				Code: "ACL_UNKNOWN_OPERATION",
				Msg:  fmt.Sprintf("unknown grant stmt operation=%s", gs.Operation().String()),
			}
		}
	}

	return nil
}

func (b *batch) executeWriteStmt(
	ctx context.Context,
	tx pgx.Tx,
	ws parsing.WriteStmt,
	addr common.Address,
	policy tableland.Policy,
	beforeRowCount int) error {
	controller, err := getController(ctx, tx, b.tp.chainID, ws.GetTableID())
	if err != nil {
		return fmt.Errorf("checking controller is set: %w", err)
	}

	if controller != "" {
		if err := b.applyPolicy(ws, policy); err != nil {
			return fmt.Errorf("not allowed to execute stmt: %w", err)
		}
	} else {
		ok, err := b.tp.acl.CheckPrivileges(ctx, tx, addr, ws.GetTableID(), ws.Operation())
		if err != nil {
			return fmt.Errorf("error checking acl: %s", err)
		}
		if !ok {
			return &txn.ErrQueryExecution{
				Code: "ACL",
				Msg:  "not enough privileges",
			}
		}
	}

	if policy.WithCheck() == "" {
		query, err := ws.GetQuery()
		if err != nil {
			return fmt.Errorf("get query query: %s", err)
		}
		cmdTag, err := tx.Exec(ctx, query)
		if err != nil {
			if code, ok := isErrCausedByQuery(err); ok {
				return &txn.ErrQueryExecution{
					Code: "POSTGRES_" + code,
					Msg:  err.Error(),
				}
			}
			return fmt.Errorf("exec query: %s", err)
		}

		if err := b.checkRowCountLimit(cmdTag, beforeRowCount); err != nil {
			return fmt.Errorf("check row limit: %w", err)
		}

		return nil
	}

	if err := ws.AddReturningClause(); err != nil {
		if err != parsing.ErrCantAddReturningOnDELETE {
			return &txn.ErrQueryExecution{
				Code: "POLICY_APPLY_RETURNING_CLAUSE",
				Msg:  err.Error(),
			}
		}
		b.tp.log.Warn().Err(err).Msg("add returning clause called on delete")
	}

	query, err := ws.GetQuery()
	if err != nil {
		return fmt.Errorf("get query: %s", err)
	}

	affectedRowsCtids, commandTag, err := b.executeQueryAndGetAffectedRows(ctx, tx, query)
	if err != nil {
		return fmt.Errorf("get rows ctids: %s", err)
	}

	if err := b.checkRowCountLimit(commandTag, beforeRowCount); err != nil {
		return fmt.Errorf("check row limit: %w", err)
	}

	// If the executed query returned ctids for the affected rows,
	// we need to execute an auditing SQL built from the policy
	// and match the result of this SQL to the number of affected rows
	sql := b.buildAuditingQueryFromPolicy(ws.GetDBTableName(), affectedRowsCtids, policy)
	if err := b.checkAffectedRowsAgainstAuditingQuery(ctx, tx, len(affectedRowsCtids), sql); err != nil {
		return fmt.Errorf("check affexted rows against auditing query: %w", err)
	}

	return nil
}

func (b *batch) applyPolicy(ws parsing.WriteStmt, policy tableland.Policy) error {
	if ws.Operation() == tableland.OpInsert && !policy.IsInsertAllowed() {
		return &txn.ErrQueryExecution{
			Code: "POLICY",
			Msg:  "insert is not allowed by policy",
		}
	}

	if ws.Operation() == tableland.OpUpdate && !policy.IsUpdateAllowed() {
		return &txn.ErrQueryExecution{
			Code: "POLICY",
			Msg:  "update is not allowed by policy",
		}
	}

	if ws.Operation() == tableland.OpDelete && !policy.IsDeleteAllowed() {
		return &txn.ErrQueryExecution{
			Code: "POLICY",
			Msg:  "delete is not allowed by policy",
		}
	}

	// the updatableColumns policy only applies to update.
	if ws.Operation() == tableland.OpUpdate {
		columnsAllowed := policy.UpdatableColumns()
		if len(columnsAllowed) > 0 {
			if err := ws.CheckColumns(columnsAllowed); err != nil {
				if err != parsing.ErrCanOnlyCheckColumnsOnUPDATE {
					return &txn.ErrQueryExecution{
						Code: "POLICY_CHECK_COLUMNS",
						Msg:  err.Error(),
					}
				}
				b.tp.log.Warn().Err(err).Msg("check columns being called on insert or delete")
			}
		}
	}

	// the whereClause policy applies to update and delete.
	if ws.Operation() == tableland.OpUpdate || ws.Operation() == tableland.OpDelete {
		if policy.WhereClause() != "" {
			if err := ws.AddWhereClause(policy.WhereClause()); err != nil {
				if err != parsing.ErrCantAddWhereOnINSERT {
					return &txn.ErrQueryExecution{
						Code: "POLICY_APPLY_WHERE_CLAUSE",
						Msg:  err.Error(),
					}
				}
				b.tp.log.Warn().Err(err).Msg("add where clause called on insert")
			}
		}
	}

	return nil
}

func (b *batch) executeQueryAndGetAffectedRows(
	ctx context.Context,
	tx pgx.Tx,
	query string) (affectedRowsCtids []string, commandTag pgconn.CommandTag, err error) {
	rows, err := tx.Query(ctx, query)
	defer func() {
		rows.Close()
		commandTag = rows.CommandTag()
	}()

	if err != nil {
		if code, ok := isErrCausedByQuery(err); ok {
			return nil, nil, &txn.ErrQueryExecution{
				Code: "POSTGRES_" + code,
				Msg:  err.Error(),
			}
		}
		return nil, nil, fmt.Errorf("exec query: %s", err)
	}

	for rows.Next() {
		var ctid pgtype.TID
		if err := rows.Scan(&ctid); err != nil {
			return nil, nil, fmt.Errorf("scan row column: %s", err)
		}

		affectedRowsCtids = append(affectedRowsCtids, fmt.Sprintf("'(%d, %d)'", ctid.BlockNumber, ctid.OffsetNumber))
	}
	return affectedRowsCtids, commandTag, nil
}

func (b *batch) checkRowCountLimit(cmdTag pgconn.CommandTag, beforeRowCount int) error {
	if b.tp.maxTableRowCount > 0 && cmdTag.Insert() {
		afterRowCount := beforeRowCount + int(cmdTag.RowsAffected())

		if afterRowCount > b.tp.maxTableRowCount {
			return &txn.ErrQueryExecution{
				Code: "ROW_COUNT_LIMIT",
				Msg:  fmt.Sprintf("table maximum row count exceeded (before %d, after %d)", beforeRowCount, afterRowCount),
			}
		}
	}

	return nil
}

func (b *batch) checkAffectedRowsAgainstAuditingQuery(
	ctx context.Context,
	tx pgx.Tx,
	affectedRowsCount int,
	sql string) error {
	var count int
	if err := tx.QueryRow(ctx, sql).Scan(&count); err != nil {
		if code, ok := isErrCausedByQuery(err); ok {
			return &txn.ErrQueryExecution{
				Code: "POSTGRES_" + code,
				Msg:  err.Error(),
			}
		}
		return fmt.Errorf("checking affected rows query exec: %s", err)
	}

	if count != affectedRowsCount {
		return &txn.ErrQueryExecution{
			Code: "POLICY_WITH_CHECK",
			Msg:  fmt.Sprintf("number of affected rows %d does not match auditing count %d", affectedRowsCount, count),
		}
	}

	return nil
}

func (b *batch) buildAuditingQueryFromPolicy(dbTableName string, ctids []string, policy tableland.Policy) string {
	return fmt.Sprintf(
		"SELECT count(1) FROM %s WHERE (%s) AND ctid in (%s) LIMIT 1",
		dbTableName,
		policy.WithCheck(),
		strings.Join(ctids, ","),
	)
}

func (b *batch) executeGrantPrivilegesTx(
	ctx context.Context,
	tx pgx.Tx,
	id tableland.TableID,
	addr common.Address,
	privileges tableland.Privileges) error {
	dbID := pgtype.Numeric{}
	if err := dbID.Set(id.String()); err != nil {
		return fmt.Errorf("parsing table id to numeric: %s", err)
	}

	// Upserts the privileges into the acl table,
	// making sure the array has unique elements.
	if _, err := tx.Exec(ctx,
		`INSERT INTO system_acl ("chain_id","table_id","controller","privileges") 
						VALUES ($1, $2, $3, $4)
						ON CONFLICT (chain_id,table_id,controller)
						DO UPDATE SET privileges = ARRAY(
							SELECT DISTINCT UNNEST(privileges || $4) 
							FROM system_acl 
							WHERE table_id = $2 AND controller = $3
						), updated_at = now();`,
		b.tp.chainID,
		dbID,
		addr.Hex(),
		privileges,
	); err != nil {
		if code, ok := isErrCausedByQuery(err); ok {
			return &txn.ErrQueryExecution{
				Code: "POSTGRES_" + code,
				Msg:  err.Error(),
			}
		}
		return fmt.Errorf("creating/updating acl entry on system acl: %s", err)
	}

	return nil
}

func (b *batch) executeRevokePrivilegesTx(
	ctx context.Context,
	tx pgx.Tx,
	id tableland.TableID,
	addr common.Address,
	privileges tableland.Privileges) error {
	dbID := pgtype.Numeric{}
	if err := dbID.Set(id.String()); err != nil {
		return fmt.Errorf("parsing table id to numeric: %s", err)
	}

	for _, privAbbr := range privileges {
		if _, err := tx.Exec(ctx,
			`UPDATE system_acl 
							SET privileges = array_remove(privileges, $4), 
								updated_at = now()
							WHERE chain_id=$1 AND table_id = $2 AND controller = $3;`,
			b.tp.chainID,
			dbID,
			addr.Hex(),
			privAbbr,
		); err != nil {
			if code, ok := isErrCausedByQuery(err); ok {
				return &txn.ErrQueryExecution{
					Code: "POSTGRES_" + code,
					Msg:  err.Error(),
				}
			}
			return fmt.Errorf("removing acl entry from system acl: %s", err)
		}
	}

	return nil
}
