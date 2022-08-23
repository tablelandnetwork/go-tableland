package db

import (
	"context"
	"database/sql"
	"fmt"
)

type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

func Prepare(ctx context.Context, db DBTX) (*Queries, error) {
	q := Queries{db: db}
	var err error
	if q.deletePendingTxByHashStmt, err = db.PrepareContext(ctx, deletePendingTxByHash); err != nil {
		return nil, fmt.Errorf("error preparing query DeletePendingTxByHash: %w", err)
	}
	if q.getAclByTableAndControllerStmt, err = db.PrepareContext(ctx, getAclByTableAndController); err != nil {
		return nil, fmt.Errorf("error preparing query GetAclByTableAndController: %w", err)
	}
	if q.getReceiptStmt, err = db.PrepareContext(ctx, getReceipt); err != nil {
		return nil, fmt.Errorf("error preparing query GetReceipt: %w", err)
	}
	if q.getSchemaByTableNameStmt, err = db.PrepareContext(ctx, getSchemaByTableName); err != nil {
		return nil, fmt.Errorf("error preparing query GetSchemaByTableName: %w", err)
	}
	if q.getTableStmt, err = db.PrepareContext(ctx, getTable); err != nil {
		return nil, fmt.Errorf("error preparing query GetTable: %w", err)
	}
	if q.getTablesByControllerStmt, err = db.PrepareContext(ctx, getTablesByController); err != nil {
		return nil, fmt.Errorf("error preparing query GetTablesByController: %w", err)
	}
	if q.getTablesByStructureStmt, err = db.PrepareContext(ctx, getTablesByStructure); err != nil {
		return nil, fmt.Errorf("error preparing query GetTablesByStructure: %w", err)
	}
	if q.insertPendingTxStmt, err = db.PrepareContext(ctx, insertPendingTx); err != nil {
		return nil, fmt.Errorf("error preparing query InsertPendingTx: %w", err)
	}
	if q.insertEVMEventStmt, err = db.PrepareContext(ctx, insertEVMEvent); err != nil {
		return nil, fmt.Errorf("error preparing query InsertEVMEvent: %w", err)
	}
	if q.getEVMEventsStmt, err = db.PrepareContext(ctx, getEVMEvents); err != nil {
		return nil, fmt.Errorf("error preparing query GetEVMEvents: %w", err)
	}
	if q.areEVMEventsPersistedStmt, err = db.PrepareContext(ctx, areEVMTxnEventsPersisted); err != nil {
		return nil, fmt.Errorf("error preparing query AreEVMEventsPersisted: %w", err)
	}
	if q.getBlocksMissingExtraInfoStmt, err = db.PrepareContext(ctx, getBlocksMissingExtraInfo); err != nil {
		return nil, fmt.Errorf("error preparing query GetBlocksMissingExtraInfo: %w", err)
	}
	if q.insertBlockExtraInfoStmt, err = db.PrepareContext(ctx, insertBlockExtraInfo); err != nil {
		return nil, fmt.Errorf("error preparing query InsertBlocksExtraInfo: %w", err)
	}
	if q.listPendingTxStmt, err = db.PrepareContext(ctx, listPendingTx); err != nil {
		return nil, fmt.Errorf("error preparing query ListPendingTx: %w", err)
	}
	return &q, nil
}

func (q *Queries) Close() error {
	var err error
	if q.deletePendingTxByHashStmt != nil {
		if cerr := q.deletePendingTxByHashStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing deletePendingTxByHashStmt: %w", cerr)
		}
	}
	if q.getAclByTableAndControllerStmt != nil {
		if cerr := q.getAclByTableAndControllerStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getAclByTableAndControllerStmt: %w", cerr)
		}
	}
	if q.getReceiptStmt != nil {
		if cerr := q.getReceiptStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getReceiptStmt: %w", cerr)
		}
	}
	if q.getSchemaByTableNameStmt != nil {
		if cerr := q.getSchemaByTableNameStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getSchemaByTableNameStmt: %w", cerr)
		}
	}
	if q.getTableStmt != nil {
		if cerr := q.getTableStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getTableStmt: %w", cerr)
		}
	}
	if q.getTablesByControllerStmt != nil {
		if cerr := q.getTablesByControllerStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getTablesByControllerStmt: %w", cerr)
		}
	}
	if q.getTablesByStructureStmt != nil {
		if cerr := q.getTablesByStructureStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getTablesByStructureStmt: %w", cerr)
		}
	}
	if q.insertPendingTxStmt != nil {
		if cerr := q.insertPendingTxStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing insertPendingTxStmt: %w", cerr)
		}
	}
	if q.insertEVMEventStmt != nil {
		if cerr := q.insertEVMEventStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing insertEVMEventStmt: %w", cerr)
		}
	}
	if q.getEVMEventsStmt != nil {
		if cerr := q.getEVMEventsStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getEVMEventsStmt: %w", cerr)
		}
	}
	if q.areEVMEventsPersistedStmt != nil {
		if cerr := q.areEVMEventsPersistedStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing areEVMEventsPersistedStmt: %w", cerr)
		}
	}
	if q.getBlocksMissingExtraInfoStmt != nil {
		if cerr := q.getBlocksMissingExtraInfoStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getBlocksMissingExtraInfo: %w", cerr)
		}
	}
	if q.insertBlockExtraInfoStmt != nil {
		if cerr := q.insertBlockExtraInfoStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing insertBlocksExtraInfo: %w", cerr)
		}
	}
	if q.listPendingTxStmt != nil {
		if cerr := q.listPendingTxStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing listPendingTxStmt: %w", cerr)
		}
	}
	return err
}

func (q *Queries) exec(ctx context.Context, stmt *sql.Stmt, query string, args ...interface{}) (sql.Result, error) {
	switch {
	case stmt != nil && q.tx != nil:
		return q.tx.StmtContext(ctx, stmt).ExecContext(ctx, args...)
	case stmt != nil:
		return stmt.ExecContext(ctx, args...)
	default:
		return q.db.ExecContext(ctx, query, args...)
	}
}

func (q *Queries) query(ctx context.Context, stmt *sql.Stmt, query string, args ...interface{}) (*sql.Rows, error) {
	switch {
	case stmt != nil && q.tx != nil:
		return q.tx.StmtContext(ctx, stmt).QueryContext(ctx, args...)
	case stmt != nil:
		return stmt.QueryContext(ctx, args...)
	default:
		return q.db.QueryContext(ctx, query, args...)
	}
}

func (q *Queries) queryRow(ctx context.Context, stmt *sql.Stmt, query string, args ...interface{}) *sql.Row {
	switch {
	case stmt != nil && q.tx != nil:
		return q.tx.StmtContext(ctx, stmt).QueryRowContext(ctx, args...)
	case stmt != nil:
		return stmt.QueryRowContext(ctx, args...)
	default:
		return q.db.QueryRowContext(ctx, query, args...)
	}
}

type Queries struct {
	db                             DBTX
	tx                             *sql.Tx
	deletePendingTxByHashStmt      *sql.Stmt
	getAclByTableAndControllerStmt *sql.Stmt
	getReceiptStmt                 *sql.Stmt
	getSchemaByTableNameStmt       *sql.Stmt
	getTableStmt                   *sql.Stmt
	getTablesByControllerStmt      *sql.Stmt
	insertPendingTxStmt            *sql.Stmt
	insertEVMEventStmt             *sql.Stmt
	getEVMEventsStmt               *sql.Stmt
	areEVMEventsPersistedStmt      *sql.Stmt
	getBlocksMissingExtraInfoStmt  *sql.Stmt
	insertBlockExtraInfoStmt       *sql.Stmt
	listPendingTxStmt              *sql.Stmt
	getTablesByStructureStmt       *sql.Stmt
}

func (q *Queries) WithTx(tx *sql.Tx) *Queries {
	return &Queries{
		db:                             tx,
		tx:                             tx,
		deletePendingTxByHashStmt:      q.deletePendingTxByHashStmt,
		getAclByTableAndControllerStmt: q.getAclByTableAndControllerStmt,
		getReceiptStmt:                 q.getReceiptStmt,
		getTableStmt:                   q.getTableStmt,
		getTablesByControllerStmt:      q.getTablesByControllerStmt,
		insertPendingTxStmt:            q.insertPendingTxStmt,
		insertEVMEventStmt:             q.insertEVMEventStmt,
		getEVMEventsStmt:               q.getEVMEventsStmt,
		areEVMEventsPersistedStmt:      q.areEVMEventsPersistedStmt,
		getBlocksMissingExtraInfoStmt:  q.getBlocksMissingExtraInfoStmt,
		insertBlockExtraInfoStmt:       q.insertBlockExtraInfoStmt,
		listPendingTxStmt:              q.listPendingTxStmt,
		getTablesByStructureStmt:       q.getTablesByStructureStmt,
	}
}
