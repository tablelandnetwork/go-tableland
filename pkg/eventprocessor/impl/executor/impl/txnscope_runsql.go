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
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
)

func (ts *txnScope) executeRunSQLEvent(
	ctx context.Context,
	e *ethereum.ContractRunSQL,
) (eventExecutionResult, error) {
	mutatingStmts, err := ts.parser.ValidateMutatingQuery(e.Statement, ts.scopeVars.ChainID)
	if err != nil {
		err := fmt.Sprintf("parsing query: %s", err)
		return eventExecutionResult{Error: &err}, nil
	}
	tableID := tables.TableID(*e.TableId)
	targetedTableID := mutatingStmts[0].GetTableID()
	if targetedTableID.ToBigInt().Cmp(tableID.ToBigInt()) != 0 {
		err := fmt.Sprintf("query targets table id %s and not %s", targetedTableID, tableID)
		return eventExecutionResult{Error: &err}, nil
	}
	if err := ts.execWriteQueries(ctx, e.Caller, mutatingStmts, e.IsOwner, &policy{e.Policy}); err != nil {
		var dbErr *errQueryExecution
		if errors.As(err, &dbErr) {
			err := fmt.Sprintf("db query execution failed (code: %s, msg: %s)", dbErr.Code, dbErr.Msg)
			return eventExecutionResult{Error: &err}, nil
		}
		return eventExecutionResult{}, fmt.Errorf("executing mutating-query: %s", err)
	}
	return eventExecutionResult{TableID: &tableID}, nil
}

func (ts *txnScope) execWriteQueries(
	ctx context.Context,
	controller common.Address,
	mqueries []parsing.MutatingStmt,
	isOwner bool,
	policy tableland.Policy,
) error {
	if len(mqueries) == 0 {
		ts.log.Warn().Msg("no mutating-queries to execute in a batch")
		return nil
	}

	dbTableName := mqueries[0].GetDBTableName()
	tablePrefix, beforeRowCount, err := getTablePrefixAndRowCountByTableID(
		ctx, ts.txn, ts.scopeVars.ChainID, mqueries[0].GetTableID(), dbTableName)
	if err != nil {
		return &errQueryExecution{
			Code: "TABLE_LOOKUP",
			Msg:  fmt.Sprintf("table prefix lookup for table id: %s", err),
		}
	}

	for _, mq := range mqueries {
		mqPrefix := mq.GetPrefix()
		if mqPrefix != "" && !strings.EqualFold(tablePrefix, mqPrefix) {
			return &errQueryExecution{
				Code: "TABLE_PREFIX",
				Msg:  fmt.Sprintf("table prefix doesn't match (exp %s, got %s)", tablePrefix, mqPrefix),
			}
		}

		switch stmt := mq.(type) {
		case parsing.GrantStmt:
			err := ts.executeGrantStmt(ctx, stmt, isOwner)
			if err != nil {
				return fmt.Errorf("executing grant stmt: %w", err)
			}
		case parsing.WriteStmt:
			if err := ts.executeWriteStmt(ctx, stmt, controller, policy, beforeRowCount); err != nil {
				return fmt.Errorf("executing write stmt: %w", err)
			}
		default:
			return fmt.Errorf("unknown stmt type")
		}
	}
	return nil
}

func (ts *txnScope) executeGrantStmt(
	ctx context.Context,
	gs parsing.GrantStmt,
	isOwner bool,
) error {
	if !isOwner {
		return &errQueryExecution{
			Code: "ACL_NOT_OWNER",
			Msg:  "non owner cannot execute grant stmt",
		}
	}

	for _, role := range gs.GetRoles() {
		switch gs.Operation() {
		case tableland.OpGrant:
			if err := ts.executeGrantPrivilegesTx(ctx, gs.GetTableID(), role, gs.GetPrivileges()); err != nil {
				return fmt.Errorf("executing grant privileges tx: %w", err)
			}
		case tableland.OpRevoke:
			if err := ts.executeRevokePrivilegesTx(ctx, gs.GetTableID(), role, gs.GetPrivileges()); err != nil {
				return fmt.Errorf("executing revoke privileges tx: %w", err)
			}
		default:
			return &errQueryExecution{
				Code: "ACL_UNKNOWN_OPERATION",
				Msg:  fmt.Sprintf("unknown grant stmt operation=%s", gs.Operation().String()),
			}
		}
	}

	return nil
}

func (ts *txnScope) executeGrantPrivilegesTx(
	ctx context.Context,
	id tables.TableID,
	addr common.Address,
	privileges tableland.Privileges,
) error {
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
	if _, err := ts.txn.ExecContext(ctx,
		`INSERT INTO system_acl ("chain_id","table_id","controller","privileges","created_at")
		 VALUES (?1, ?2, ?3, ?4, ?5)
		 ON CONFLICT (chain_id,table_id,controller)
		 DO UPDATE SET privileges = privileges | ?4, updated_at = ?5`,
		ts.scopeVars.ChainID,
		id.ToBigInt().Int64(),
		addr.Hex(),
		privilegesMask,
		time.Now().Unix()); err != nil {
		if code, ok := isErrCausedByQuery(err); ok {
			return &errQueryExecution{
				Code: "SQLITE_" + code,
				Msg:  err.Error(),
			}
		}
		return fmt.Errorf("creating/updating acl entry on system acl: %s", err)
	}

	return nil
}

func (ts *txnScope) executeRevokePrivilegesTx(
	ctx context.Context,
	id tables.TableID,
	addr common.Address,
	privileges tableland.Privileges,
) error {
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

	if _, err := ts.txn.ExecContext(ctx,
		`UPDATE system_acl 
	     SET privileges = privileges & ?4, updated_at = ?5
		 WHERE chain_id=?1 AND table_id = ?2 AND controller = ?3`,
		ts.scopeVars.ChainID,
		id.String(),
		addr.Hex(),
		privilegesMask,
		time.Now().Unix(),
	); err != nil {
		if code, ok := isErrCausedByQuery(err); ok {
			return &errQueryExecution{
				Code: "SQLITE_" + code,
				Msg:  err.Error(),
			}
		}
		return fmt.Errorf("removing acl entry from system acl: %s", err)
	}

	return nil
}

func (ts *txnScope) executeWriteStmt(
	ctx context.Context,
	ws parsing.WriteStmt,
	addr common.Address,
	policy tableland.Policy,
	beforeRowCount int,
) error {
	controller, err := ts.getController(ctx, ws.GetTableID())
	if err != nil {
		return fmt.Errorf("checking controller is set: %w", err)
	}

	if controller != "" {
		if err := ts.applyPolicy(ws, policy); err != nil {
			return fmt.Errorf("not allowed to execute stmt: %w", err)
		}
	} else {
		ok, err := ts.acl.CheckPrivileges(ctx, ts.txn, addr, ws.GetTableID(), ws.Operation())
		if err != nil {
			return fmt.Errorf("error checking acl: %s", err)
		}
		if !ok {
			return &errQueryExecution{
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
		cmdTag, err := ts.txn.ExecContext(ctx, query)
		if err != nil {
			if code, ok := isErrCausedByQuery(err); ok {
				return &errQueryExecution{
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
		if err := ts.checkRowCountLimit(ra, isInsert, beforeRowCount); err != nil {
			return fmt.Errorf("check row limit: %w", err)
		}

		return nil
	}

	if err := ws.AddReturningClause(); err != nil {
		if err != parsing.ErrCantAddReturningOnDELETE {
			return &errQueryExecution{
				Code: "POLICY_APPLY_RETURNING_CLAUSE",
				Msg:  err.Error(),
			}
		}
		ts.log.Warn().Err(err).Msg("add returning clause called on delete")
	}

	query, err := ws.GetQuery()
	if err != nil {
		return fmt.Errorf("get query: %s", err)
	}

	affectedRowIDs, err := ts.executeQueryAndGetAffectedRows(ctx, query)
	if err != nil {
		return fmt.Errorf("get rows ids: %s", err)
	}

	isInsert := ws.Operation() == tableland.OpInsert
	if err := ts.checkRowCountLimit(int64(len(affectedRowIDs)), isInsert, beforeRowCount); err != nil {
		return fmt.Errorf("check row limit: %w", err)
	}

	// If the executed query returned rowids for the affected rows,
	// we need to execute an auditing SQL built from the policy
	// and match the result of this SQL to the number of affected rows
	sql := buildAuditingQueryFromPolicy(ws.GetDBTableName(), affectedRowIDs, policy)
	if err := ts.checkAffectedRowsAgainstAuditingQuery(ctx, len(affectedRowIDs), sql); err != nil {
		return fmt.Errorf("check affected rows against auditing query: %w", err)
	}

	return nil
}

func (ts *txnScope) checkAffectedRowsAgainstAuditingQuery(
	ctx context.Context,
	affectedRowsCount int,
	sql string,
) error {
	var count int
	if err := ts.txn.QueryRowContext(ctx, sql).Scan(&count); err != nil {
		if code, ok := isErrCausedByQuery(err); ok {
			return &errQueryExecution{
				Code: "SQLITE_" + code,
				Msg:  err.Error(),
			}
		}
		return fmt.Errorf("checking affected rows query exec: %s", err)
	}

	if count != affectedRowsCount {
		return &errQueryExecution{
			Code: "POLICY_WITH_CHECK",
			Msg:  fmt.Sprintf("number of affected rows %d does not match auditing count %d", affectedRowsCount, count),
		}
	}

	return nil
}

func (ts *txnScope) executeQueryAndGetAffectedRows(
	ctx context.Context,
	query string,
) (affectedRowIDs []int64, err error) {
	rows, err := ts.txn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("executing query: %s", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			ts.log.Warn().Err(err).Msg("closing rows")
		}
	}()

	if err != nil {
		if code, ok := isErrCausedByQuery(err); ok {
			return nil, &errQueryExecution{
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

func (ts *txnScope) checkRowCountLimit(rowsAffected int64, isInsert bool, beforeRowCount int) error {
	if ts.scopeVars.MaxTableRowCount > 0 && isInsert {
		afterRowCount := beforeRowCount + int(rowsAffected)

		if afterRowCount > ts.scopeVars.MaxTableRowCount {
			return &errQueryExecution{
				Code: "ROW_COUNT_LIMIT",
				Msg:  fmt.Sprintf("table maximum row count exceeded (before %d, after %d)", beforeRowCount, afterRowCount),
			}
		}
	}

	return nil
}

func (ts *txnScope) applyPolicy(ws parsing.WriteStmt, policy tableland.Policy) error {
	if ws.Operation() == tableland.OpInsert && !policy.IsInsertAllowed() {
		return &errQueryExecution{
			Code: "POLICY",
			Msg:  "insert is not allowed by policy",
		}
	}

	if ws.Operation() == tableland.OpUpdate && !policy.IsUpdateAllowed() {
		return &errQueryExecution{
			Code: "POLICY",
			Msg:  "update is not allowed by policy",
		}
	}

	if ws.Operation() == tableland.OpDelete && !policy.IsDeleteAllowed() {
		return &errQueryExecution{
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
					return &errQueryExecution{
						Code: "POLICY_CHECK_COLUMNS",
						Msg:  err.Error(),
					}
				}
				ts.log.Warn().Err(err).Msg("check columns being called on insert or delete")
			}
		}
	}

	// the whereClause policy applies to update and delete.
	if ws.Operation() == tableland.OpUpdate || ws.Operation() == tableland.OpDelete {
		if policy.WhereClause() != "" {
			if err := ws.AddWhereClause(policy.WhereClause()); err != nil {
				if err != parsing.ErrCantAddWhereOnINSERT {
					return &errQueryExecution{
						Code: "POLICY_APPLY_WHERE_CLAUSE",
						Msg:  err.Error(),
					}
				}
				ts.log.Warn().Err(err).Msg("add where clause called on insert")
			}
		}
	}

	return nil
}

// getController gets the controller for a given table.
func (ts *txnScope) getController(
	ctx context.Context,
	tableID tables.TableID,
) (string, error) {
	q := "SELECT controller FROM system_controller where chain_id=?1 AND table_id=?2"
	r := ts.txn.QueryRowContext(ctx, q, ts.scopeVars.ChainID, tableID.ToBigInt().Uint64())
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

// getTablePrefixAndRowCountByTableID returns the table prefix and current row count for a TableID
// within the provided transaction.
func getTablePrefixAndRowCountByTableID(
	ctx context.Context,
	tx *sql.Tx,
	chainID tableland.ChainID,
	tableID tables.TableID,
	dbTableName string,
) (string, int, error) {
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

type policy struct {
	ethereum.ITablelandControllerPolicy
}

func (p *policy) IsInsertAllowed() bool {
	return p.ITablelandControllerPolicy.AllowInsert
}

func (p *policy) IsUpdateAllowed() bool {
	return p.ITablelandControllerPolicy.AllowUpdate
}

func (p *policy) IsDeleteAllowed() bool {
	return p.ITablelandControllerPolicy.AllowDelete
}

func (p *policy) WhereClause() string {
	return p.ITablelandControllerPolicy.WhereClause
}

func (p *policy) UpdatableColumns() []string {
	return p.ITablelandControllerPolicy.UpdatableColumns
}

func (p *policy) WithCheck() string {
	return p.ITablelandControllerPolicy.WithCheck
}
