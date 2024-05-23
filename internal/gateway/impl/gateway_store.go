package impl

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/tablelandnetwork/sqlparser"
	"github.com/textileio/go-tableland/internal/gateway"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/database"
	"github.com/textileio/go-tableland/pkg/database/db"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/tables"
)

// GatewayStore is the storage layer of the gateway.
type GatewayStore struct {
	db *database.SQLiteDB
}

// NewGatewayStore creates a new GatewayStore.
func NewGatewayStore(db *database.SQLiteDB) *GatewayStore {
	return &GatewayStore{
		db: db,
	}
}

// Read executes a parsed read statement.
func (s *GatewayStore) Read(
	ctx context.Context, stmt parsing.ReadStmt, resolver sqlparser.ReadStatementResolver,
) (*gateway.TableData, error) {
	query, err := stmt.GetQuery(resolver)
	if err != nil {
		return nil, fmt.Errorf("get query: %s", err)
	}
	ret, err := s.execReadQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("parsing result to json: %s", err)
	}

	return ret, nil
}

// GetTable returns a table information.
func (s *GatewayStore) GetTable(
	ctx context.Context, chainID tableland.ChainID, tableID tables.TableID,
) (gateway.Table, error) {
	table, err := s.db.Queries.GetTable(ctx, db.GetTableParams{
		ChainID: int64(chainID),
		ID:      tableID.ToBigInt().Int64(),
	})
	if err == sql.ErrNoRows {
		return gateway.Table{}, fmt.Errorf("not found: %w", err)
	}
	if err != nil {
		return gateway.Table{}, fmt.Errorf("getting table: %s", err)
	}

	tableID, err = tables.NewTableIDFromInt64(table.ID)
	if err != nil {
		return gateway.Table{}, fmt.Errorf("table id from int64: %s", err)
	}

	return gateway.Table{
		ID:         tableID,
		ChainID:    tableland.ChainID(table.ChainID),
		Controller: table.Controller,
		Prefix:     table.Prefix,
		Structure:  table.Structure,
		CreatedAt:  time.Unix(table.CreatedAt, 0),
	}, nil
}

// GetSchemaByTableName returns the table schema given its name.
func (s *GatewayStore) GetSchemaByTableName(ctx context.Context, tblName string) (gateway.TableSchema, error) {
	createStmt, err := s.db.Queries.GetSchemaByTableName(ctx, tblName)
	if err != nil {
		return gateway.TableSchema{}, fmt.Errorf("failed to get the table: %s", err)
	}

	if strings.Contains(strings.ToLower(createStmt), "autoincrement") {
		createStmt = strings.Replace(createStmt, "autoincrement", "", -1)
	}

	index := strings.LastIndex(strings.ToLower(createStmt), "strict")
	ast, err := sqlparser.Parse(createStmt[:index])
	if err != nil {
		return gateway.TableSchema{}, fmt.Errorf("failed to parse create stmt: %s", err)
	}

	if ast.Errors[0] != nil {
		return gateway.TableSchema{}, fmt.Errorf("non-syntax error: %s", ast.Errors[0])
	}

	createTableNode := ast.Statements[0].(*sqlparser.CreateTable)
	columns := make([]gateway.ColumnSchema, len(createTableNode.ColumnsDef))
	for i, col := range createTableNode.ColumnsDef {
		colConstraints := []string{}
		for _, colConstraint := range col.Constraints {
			colConstraints = append(colConstraints, colConstraint.String())
		}

		columns[i] = gateway.ColumnSchema{
			Name:        col.Column.String(),
			Type:        strings.ToLower(col.Type),
			Constraints: colConstraints,
		}
	}

	tableConstraints := make([]string, len(createTableNode.Constraints))
	for i, tableConstraint := range createTableNode.Constraints {
		tableConstraints[i] = tableConstraint.String()
	}

	return gateway.TableSchema{
		Columns:          columns,
		TableConstraints: tableConstraints,
	}, nil
}

// GetReceipt gets the receipt of a given transaction hash.
func (s *GatewayStore) GetReceipt(
	ctx context.Context, chainID tableland.ChainID, txnHash string,
) (gateway.Receipt, bool, error) {
	params := db.GetReceiptParams{
		ChainID: int64(chainID),
		TxnHash: txnHash,
	}

	res, err := s.db.Queries.GetReceipt(ctx, params)
	if err == sql.ErrNoRows {
		return gateway.Receipt{}, false, nil
	}
	if err != nil {
		return gateway.Receipt{}, false, fmt.Errorf("get receipt: %s", err)
	}

	receipt := gateway.Receipt{
		ChainID:      chainID,
		BlockNumber:  res.BlockNumber,
		IndexInBlock: res.IndexInBlock,
		TxnHash:      txnHash,
	}

	if res.Error.Valid {
		receipt.Error = &res.Error.String

		errorEventIdx := int(res.ErrorEventIdx.Int64)
		receipt.ErrorEventIdx = &errorEventIdx
	}

	if res.TableID.Valid {
		id, err := tables.NewTableIDFromInt64(res.TableID.Int64)
		if err != nil {
			return gateway.Receipt{}, false, fmt.Errorf("parsing id integer: %s", err)
		}
		receipt.TableID = &id // nolint
	}

	if res.TableIds.Valid {
		tableIdsStr := strings.Split(res.TableIds.String, ",")
		tableIds := make([]tables.TableID, len(tableIdsStr))
		for i, idStr := range tableIdsStr {
			tableID, err := tables.NewTableID(idStr)
			if err != nil {
				return gateway.Receipt{}, false, fmt.Errorf("parsing id string: %s", err)
			}
			tableIds[i] = tableID
		}
		receipt.TableIDs = tableIds
	}

	return receipt, true, nil
}

func (s *GatewayStore) execReadQuery(ctx context.Context, q string) (*gateway.TableData, error) {
	rows, err := s.db.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("executing query: %s", err)
	}
	defer func() {
		if err = rows.Close(); err != nil {
			s.db.Log.Warn().Err(err).Msg("closing rows")
		}
	}()
	return rowsToTableData(rows)
}
