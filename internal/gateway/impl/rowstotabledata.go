package impl

import (
	"database/sql"
	"fmt"

	"github.com/textileio/go-tableland/internal/gateway"
)

func rowsToTableData(rows *sql.Rows) (*gateway.TableData, error) {
	columns, err := getColumnsData(rows)
	if err != nil {
		return nil, fmt.Errorf("get columns from rows: %s", err)
	}
	rowsData, err := getRowsData(rows, len(columns))
	if err != nil {
		return nil, err
	}

	return &gateway.TableData{
		Columns: columns,
		Rows:    rowsData,
	}, nil
}

func getColumnsData(rows *sql.Rows) ([]gateway.Column, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns from sql.Rows: %s", err)
	}
	columns := make([]gateway.Column, len(cols))
	for i := range cols {
		columns[i] = gateway.Column{Name: cols[i]}
	}
	return columns, nil
}

func getRowsData(rows *sql.Rows, numColumns int) ([][]*gateway.ColumnValue, error) {
	rowsData := make([][]*gateway.ColumnValue, 0)
	for rows.Next() {
		vals := make([]*gateway.ColumnValue, numColumns)
		for i := range vals {
			val := &gateway.ColumnValue{}
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
