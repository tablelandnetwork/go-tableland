package impl

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mattn/go-sqlite3"
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
	pool    *sql.DB
	chBatch chan struct{}

	maxTableRowCount int
	acl              tableland.ACL
}

var _ txn.TxnProcessor = (*TblTxnProcessor)(nil)

// NewTxnProcessor returns a new Tableland transaction processor.
func NewTxnProcessor(
	chainID tableland.ChainID,
	dbURI string,
	maxTableRowCount int,
	acl tableland.ACL) (*TblTxnProcessor, error) {
	pool, err := sql.Open("sqlite3", dbURI)
	if err != nil {
		return nil, fmt.Errorf("connecting to db: %s", err)
	}
	pool.SetMaxOpenConns(1)
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

	ops := sql.TxOptions{
		Isolation: sql.LevelSerializable,
		ReadOnly:  false,
	}
	txn, err := tp.pool.BeginTx(ctx, &ops)
	if err != nil {
		tp.chBatch <- struct{}{}
		return nil, fmt.Errorf("opening db transaction: %s", err)
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
	txn *sql.Tx
	tp  *TblTxnProcessor
}

func runWithinSavepoint(ctx context.Context, txn *sql.Tx, f func(ctx context.Context, txn *sql.Tx) error) error {
	if _, err := txn.ExecContext(ctx, "SAVEPOINT tbl"); err != nil {
		return fmt.Errorf("creating savepoint: %s", err)
	}
	if err := f(ctx, txn); err != nil {
		if _, err := txn.ExecContext(ctx, "ROLLBACK TO tbl"); err != nil {
			return fmt.Errorf("rollbacking savepoint: %s", err)
		}
		return fmt.Errorf("executing query: %w", err)
	}
	if _, err := txn.ExecContext(ctx, "RELEASE SAVEPOINT tbl"); err != nil {
		return fmt.Errorf("releasing savepoint: %s", err)
	}

	return nil
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
	f := func(ctx context.Context, txn *sql.Tx) error {
		if _, err := txn.ExecContext(ctx,
			`INSERT INTO registry ("chain_id", "id","controller","prefix","structure") 
		  	 VALUES (?1,?2,?3,?4,?5);`,
			b.tp.chainID,
			id.String(),
			controller,
			createStmt.GetPrefix(),
			createStmt.GetStructureHash()); err != nil {
			return fmt.Errorf("inserting new table in system-wide registry: %s", err)
		}

		if _, err := txn.ExecContext(ctx,
			`INSERT INTO system_acl ("chain_id","table_id","controller","privileges") 
			 VALUES (?1,?2,?3,?4);`,
			b.tp.chainID,
			id.String(),
			controller,
			tableland.PrivInsert.Bitfield|tableland.PrivUpdate.Bitfield|tableland.PrivDelete.Bitfield,
		); err != nil {
			return fmt.Errorf("inserting new entry into system acl: %s", err)
		}

		query, err := createStmt.GetRawQueryForTableID(id)
		if err != nil {
			return fmt.Errorf("get query for table id: %s", err)
		}
		if _, err := txn.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("exec CREATE statement: %s", err)
		}

		return nil
	}
	if err := runWithinSavepoint(ctx, b.txn, f); err != nil {
		return fmt.Errorf("running within savepoint: %w", err)
	}
	return nil
}

func (b *batch) ExecWriteQueries(
	ctx context.Context,
	controller common.Address,
	mqueries []parsing.MutatingStmt,
	isOwner bool,
	policy tableland.Policy) error {
	if len(mqueries) == 0 {
		b.tp.log.Warn().Msg("no mutating-queries to execute in a batch")
		return nil
	}

	f := func(ctx context.Context, tx *sql.Tx) error {
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
	if err := runWithinSavepoint(ctx, b.txn, f); err != nil {
		return fmt.Errorf("running within savepoint: %w", err)
	}

	return nil
}

// SetController sets and unsets the controller of a table.
func (b *batch) SetController(
	ctx context.Context,
	id tableland.TableID,
	controller common.Address) error {
	f := func(ctx context.Context, tx *sql.Tx) error {
		if controller == common.HexToAddress("0x0") {
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM system_controller WHERE chain_id = ?1 AND table_id = ?2;`,
				b.tp.chainID,
				id.String(),
			); err != nil {
				if code, ok := isErrCausedByQuery(err); ok {
					return &txn.ErrQueryExecution{
						Code: "SQLITE_" + code,
						Msg:  err.Error(),
					}
				}
				return fmt.Errorf("deleting entry from system controller: %s", err)
			}
		} else {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO system_controller ("chain_id", "table_id", "controller") 
				VALUES (?1, ?2, ?3)
				ON CONFLICT ("chain_id", "table_id")
				DO UPDATE set controller = ?3;`,
				b.tp.chainID,
				id.String(),
				controller.Hex(),
			); err != nil {
				if code, ok := isErrCausedByQuery(err); ok {
					return &txn.ErrQueryExecution{
						Code: "SQLITE_" + code,
						Msg:  err.Error(),
					}
				}
				return fmt.Errorf("inserting new entry into system controller: %s", err)
			}
		}
		return nil
	}

	if err := runWithinSavepoint(ctx, b.txn, f); err != nil {
		return fmt.Errorf("running within savepoint: %w", err)
	}

	return nil
}

// GrantPrivileges gives privileges to an address on a table.
func (b *batch) GrantPrivileges(
	ctx context.Context,
	id tableland.TableID,
	addr common.Address,
	privileges tableland.Privileges) error {
	f := func(ctx context.Context, tx *sql.Tx) error {
		if err := b.executeGrantPrivilegesTx(ctx, tx, id, addr, privileges); err != nil {
			return fmt.Errorf("executing grant privileges tx: %w", err)
		}
		return nil
	}
	if err := runWithinSavepoint(ctx, b.txn, f); err != nil {
		return fmt.Errorf("running within savepoint: %w", err)
	}
	return nil
}

// RevokePrivileges revokes privileges from an address on a table.
func (b *batch) RevokePrivileges(
	ctx context.Context,
	id tableland.TableID,
	addr common.Address,
	privileges tableland.Privileges) error {
	f := func(ctx context.Context, tx *sql.Tx) error {
		if err := b.executeRevokePrivilegesTx(ctx, tx, id, addr, privileges); err != nil {
			return fmt.Errorf("executing revoke privileges tx: %w", err)
		}
		return nil
	}
	if err := runWithinSavepoint(ctx, b.txn, f); err != nil {
		return fmt.Errorf("running within savepoint: %w", err)
	}
	return nil
}

func (b *batch) GetLastProcessedHeight(ctx context.Context) (int64, error) {
	var blockNumber int64
	f := func(ctx context.Context, tx *sql.Tx) error {
		r := tx.QueryRowContext(ctx, "SELECT block_number FROM system_txn_processor WHERE chain_id=?1 LIMIT 1", b.tp.chainID)
		if err := r.Scan(&blockNumber); err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return fmt.Errorf("get last block number query: %s", err)
		}
		return nil
	}
	if err := runWithinSavepoint(ctx, b.txn, f); err != nil {
		return 0, fmt.Errorf("running within savepoint: %w", err)
	}
	return blockNumber, nil
}

func (b *batch) SetLastProcessedHeight(ctx context.Context, height int64) error {
	f := func(ctx context.Context, tx *sql.Tx) error {
		tag, err := tx.ExecContext(
			ctx,
			"UPDATE system_txn_processor SET block_number=?1 WHERE chain_id=?2", height, b.tp.chainID)
		if err != nil {
			return fmt.Errorf("update last processed block number: %s", err)
		}
		ra, err := tag.RowsAffected()
		if err != nil {
			return fmt.Errorf("rows affected: %s", err)
		}
		if ra != 1 {
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO system_txn_processor (block_number, chain_id) VALUES (?1, ?2)",
				height,
				b.tp.chainID,
			); err != nil {
				return fmt.Errorf("inserting first processed height: %s", err)
			}
		}
		return nil
	}
	if err := runWithinSavepoint(ctx, b.txn, f); err != nil {
		return fmt.Errorf("running within savepoint: %w", err)
	}
	return nil
}

func (b *batch) SaveTxnReceipts(ctx context.Context, rs []eventprocessor.Receipt) error {
	f := func(ctx context.Context, tx *sql.Tx) error {
		for _, r := range rs {
			tableID := sql.NullInt64{Valid: false}
			if r.TableID != nil {
				tableID.Valid = true
				tableID.Int64 = r.TableID.ToBigInt().Int64()
			}
			if r.Error != nil {
				*r.Error = strings.ToValidUTF8(*r.Error, "")
			}
			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO system_txn_receipts (chain_id,txn_hash,error,table_id,block_number,index_in_block) 
				 VALUES (?1,?2,?3,?4,?5,?6)`,
				r.ChainID, r.TxnHash, r.Error, tableID, r.BlockNumber, r.IndexInBlock); err != nil {
				return fmt.Errorf("insert txn receipt: %s", err)
			}
		}
		return nil
	}
	if err := runWithinSavepoint(ctx, b.txn, f); err != nil {
		return fmt.Errorf("running within savepoint: %w", err)
	}
	return nil
}

func (b *batch) TxnReceiptExists(ctx context.Context, txnHash common.Hash) (bool, error) {
	var exists bool
	f := func(ctx context.Context, tx *sql.Tx) error {
		r := tx.QueryRowContext(
			ctx,
			`SELECT 1 from system_txn_receipts WHERE chain_id=?1 and txn_hash=?2`,
			b.tp.chainID, txnHash.Hex())
		var dummy int
		err := r.Scan(&dummy)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return fmt.Errorf("get txn receipt: %s", err)
		}
		exists = true
		return nil
	}
	if err := runWithinSavepoint(ctx, b.txn, f); err != nil {
		return false, fmt.Errorf("running within savepoint: %w", err)
	}
	return exists, nil
}

// Close closes gracefully the batch. Clients should *always* `defer Close()` when
// opening batches.
func (b *batch) Close() error {
	defer func() { b.tp.chBatch <- struct{}{} }()

	// Calling rollback is always safe:
	// - If Commit() wasn't called, the result is a rollback.
	// - If Commit() was called, *sql.Txn guarantees is a noop.
	if err := b.txn.Rollback(); err != nil {
		if err != sql.ErrTxDone {
			return fmt.Errorf("closing batch: %s", err)
		}
	}

	return nil
}

func (b *batch) Commit() error {
	if err := b.txn.Commit(); err != nil {
		return fmt.Errorf("commit txn: %s", err)
	}
	return nil
}

// GetTablePrefixAndRowCountByTableID returns the table prefix and current row count for a TableID
// within the provided transaction.
func GetTablePrefixAndRowCountByTableID(
	ctx context.Context,
	tx *sql.Tx,
	chainID tableland.ChainID,
	tableID tableland.TableID,
	dbTableName string) (string, int, error) {
	q := fmt.Sprintf(
		"SELECT (SELECT prefix FROM registry where chain_id=?1 AND id=?2), (SELECT count(*) FROM %s)", dbTableName)
	r := tx.QueryRowContext(ctx, q, chainID, tableID.String())

	var tablePrefix string
	var rowCount int
	err := r.Scan(&tablePrefix, &rowCount)
	if err == sql.ErrNoRows {
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
	tx *sql.Tx,
	chainID tableland.ChainID,
	tableID tableland.TableID) (string, error) {
	q := "SELECT controller FROM system_controller where chain_id=?1 AND table_id=?2"
	r := tx.QueryRowContext(ctx, q, chainID, tableID.ToBigInt().Uint64())
	var controller string
	err := r.Scan(&controller)
	if err == sql.ErrNoRows {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("controller lookup: %s", err)
	}
	return controller, nil
}

func (b *batch) executeGrantStmt(
	ctx context.Context,
	tx *sql.Tx,
	gs parsing.GrantStmt,
	isOwner bool) error {
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
	tx *sql.Tx,
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
		cmdTag, err := tx.ExecContext(ctx, query)
		if err != nil {
			if code, ok := isErrCausedByQuery(err); ok {
				return &txn.ErrQueryExecution{
					Code: "SQLITE_" + code,
					Msg:  err.Error(),
				}
			}
			return fmt.Errorf("exec query: %s", err)
		}

		ra, err := cmdTag.RowsAffected()
		if err != nil {
			return fmt.Errorf("get rows affected: %s", err)
		}

		isInsert := ws.Operation() == tableland.OpInsert
		if err := b.checkRowCountLimit(ra, isInsert, beforeRowCount); err != nil {
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

	affectedRowIDs, err := b.executeQueryAndGetAffectedRows(ctx, tx, query)
	if err != nil {
		return fmt.Errorf("get rows ids: %s", err)
	}

	isInsert := ws.Operation() == tableland.OpInsert
	if err := b.checkRowCountLimit(int64(len(affectedRowIDs)), isInsert, beforeRowCount); err != nil {
		return fmt.Errorf("check row limit: %w", err)
	}

	// If the executed query returned rowids for the affected rows,
	// we need to execute an auditing SQL built from the policy
	// and match the result of this SQL to the number of affected rows
	sql := buildAuditingQueryFromPolicy(ws.GetDBTableName(), affectedRowIDs, policy)
	if err := checkAffectedRowsAgainstAuditingQuery(ctx, tx, len(affectedRowIDs), sql); err != nil {
		return fmt.Errorf("check affected rows against auditing query: %w", err)
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
	tx *sql.Tx,
	query string) (affectedRowIDs []int64, err error) {
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("executing query: %s", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			b.tp.log.Warn().Err(err).Msg("closing rows")
		}
	}()

	if err != nil {
		if code, ok := isErrCausedByQuery(err); ok {
			return nil, &txn.ErrQueryExecution{
				Code: "SQLITE_" + code,
				Msg:  err.Error(),
			}
		}
		return nil, fmt.Errorf("exec query: %s", err)
	}

	for rows.Next() {
		var rowID int64
		if err := rows.Scan(&rowID); err != nil {
			return nil, fmt.Errorf("scan row column: %s", err)
		}

		affectedRowIDs = append(affectedRowIDs, rowID)
	}
	return affectedRowIDs, nil
}

func (b *batch) checkRowCountLimit(rowsAffected int64, isInsert bool, beforeRowCount int) error {
	if b.tp.maxTableRowCount > 0 && isInsert {
		afterRowCount := beforeRowCount + int(rowsAffected)

		if afterRowCount > b.tp.maxTableRowCount {
			return &txn.ErrQueryExecution{
				Code: "ROW_COUNT_LIMIT",
				Msg:  fmt.Sprintf("table maximum row count exceeded (before %d, after %d)", beforeRowCount, afterRowCount),
			}
		}
	}

	return nil
}

func checkAffectedRowsAgainstAuditingQuery(
	ctx context.Context,
	tx *sql.Tx,
	affectedRowsCount int,
	sql string) error {
	var count int
	if err := tx.QueryRowContext(ctx, sql).Scan(&count); err != nil {
		if code, ok := isErrCausedByQuery(err); ok {
			return &txn.ErrQueryExecution{
				Code: "SQLITE_" + code,
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

func buildAuditingQueryFromPolicy(dbTableName string, rowIDs []int64, policy tableland.Policy) string {
	ids := make([]string, len(rowIDs))
	for i, id := range rowIDs {
		ids[i] = strconv.FormatInt(id, 10)
	}
	return fmt.Sprintf(
		"SELECT count(1) FROM %s WHERE (%s) AND rowid in (%s) LIMIT 1",
		dbTableName,
		policy.WithCheck(),
		strings.Join(ids, ","),
	)
}

func (b *batch) executeGrantPrivilegesTx(
	ctx context.Context,
	tx *sql.Tx,
	id tableland.TableID,
	addr common.Address,
	privileges tableland.Privileges) error {
	var privilegesMask int
	for _, privilege := range privileges {
		switch privilege {
		case tableland.PrivInsert:
			privilegesMask |= tableland.PrivInsert.Bitfield
		case tableland.PrivUpdate:
			privilegesMask |= tableland.PrivUpdate.Bitfield
		case tableland.PrivDelete:
			privilegesMask |= tableland.PrivDelete.Bitfield
		default:
			return fmt.Errorf("unknown privilege: %s", privilege.Abbreviation)
		}
	}

	// Upserts the privileges into the acl table,
	// making sure the array has unique elements.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO system_acl ("chain_id","table_id","controller","privileges","created_at")
		 VALUES (?1, ?2, ?3, ?4, ?5)
		 ON CONFLICT (chain_id,table_id,controller)
		 DO UPDATE SET privileges = privileges | ?4, updated_at = ?5`,
		b.tp.chainID,
		id.ToBigInt().Int64(),
		addr.Hex(),
		privilegesMask,
		time.Now().Unix()); err != nil {
		if code, ok := isErrCausedByQuery(err); ok {
			return &txn.ErrQueryExecution{
				Code: "SQLITE_" + code,
				Msg:  err.Error(),
			}
		}
		return fmt.Errorf("creating/updating acl entry on system acl: %s", err)
	}

	return nil
}

func (b *batch) executeRevokePrivilegesTx(
	ctx context.Context,
	tx *sql.Tx,
	id tableland.TableID,
	addr common.Address,
	privileges tableland.Privileges) error {
	privilegesMask := tableland.PrivInsert.Bitfield | tableland.PrivUpdate.Bitfield | tableland.PrivDelete.Bitfield
	// Tune the mask to have a 0 in the places we want to disable the bit.
	// For example, if we want to remove tableland.PrivUpdate, the following
	// code will transform 111 -> 101.
	// We'll then use 101 to AND the value in the DB.
	for _, privilege := range privileges {
		switch privilege {
		case tableland.PrivInsert:
			privilegesMask &^= tableland.PrivInsert.Bitfield
		case tableland.PrivUpdate:
			privilegesMask &^= tableland.PrivUpdate.Bitfield
		case tableland.PrivDelete:
			privilegesMask &^= tableland.PrivDelete.Bitfield
		default:
			return fmt.Errorf("unknown privilege: %s", privilege.Abbreviation)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE system_acl 
	     SET privileges = privileges & ?4, updated_at = ?5
		 WHERE chain_id=?1 AND table_id = ?2 AND controller = ?3`,
		b.tp.chainID,
		id.String(),
		addr.Hex(),
		privilegesMask,
		time.Now().Unix(),
	); err != nil {
		if code, ok := isErrCausedByQuery(err); ok {
			return &txn.ErrQueryExecution{
				Code: "SQLITE_" + code,
				Msg:  err.Error(),
			}
		}
		return fmt.Errorf("removing acl entry from system acl: %s", err)
	}

	return nil
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
