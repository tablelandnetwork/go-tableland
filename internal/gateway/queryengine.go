package gateway

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/marcboeker/go-duckdb"
	"github.com/tablelandnetwork/sqlparser"
)

type QueryEngine struct {
	db *sql.DB
}

func NewQueryEngine() (*QueryEngine, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %s", err)
	}

	_, err = db.Exec("INSTALL https; LOAD https;")
	if err != nil {
		return nil, fmt.Errorf("install https: %s", err)
	}

	return &QueryEngine{
		db: db,
	}, nil
}

func (qe *QueryEngine) Query(ctx context.Context, sql string) (*TableData, error) {
	ast, err := sqlparser.Parse(sql)
	if err != nil {
		return nil, err
	}
	ast.Statements[0].(*sqlparser.Select).From.(*sqlparser.AliasedTableExpr).Expr.(*sqlparser.Table).Name = sqlparser.Identifier(fmt.Sprintf("read_parquet(['http://34.106.97.87:8002/v1/os/t2gh7m2iaqvwv2oexsaetncmdm6hhac7k6ne3teda/%s'])", ast.Statements[0].(*sqlparser.Select).From.(*sqlparser.AliasedTableExpr).Expr.(*sqlparser.Table).Name.String()))
	rows, err := qe.db.QueryContext(ctx, ast.String())
	if err != nil {
		return nil, fmt.Errorf("executing query: %s", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	return rowsToTableData(rows)
}

func (qe *QueryEngine) Close() error {
	return qe.db.Close()
}

func rowsToTableData(rows *sql.Rows) (*TableData, error) {
	columns, err := getColumnsData(rows)
	if err != nil {
		return nil, fmt.Errorf("get columns from rows: %s", err)
	}
	rowsData, err := getRowsData(rows, len(columns))
	if err != nil {
		return nil, err
	}

	return &TableData{
		Columns: columns,
		Rows:    rowsData,
	}, nil
}

func getColumnsData(rows *sql.Rows) ([]Column, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns from sql.Rows: %s", err)
	}
	columns := make([]Column, len(cols))
	for i := range cols {
		columns[i] = Column{Name: cols[i]}
	}
	return columns, nil
}

func getRowsData(rows *sql.Rows, numColumns int) ([][]*ColumnValue, error) {
	rowsData := make([][]*ColumnValue, 0)
	for rows.Next() {
		vals := make([]*ColumnValue, numColumns)
		for i := range vals {
			val := &ColumnValue{}
			vals[i] = val
		}
		scanArgs := make([]interface{}, len(vals))
		for i := range vals {
			scanArgs[i] = vals[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("scan row column: %s", err)
		}
		rowsData = append(rowsData, vals)
	}
	return rowsData, nil
}
