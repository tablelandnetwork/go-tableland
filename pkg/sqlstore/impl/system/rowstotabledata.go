package system

import (
	"database/sql"
	"fmt"

	"github.com/textileio/go-tableland/internal/tableland"
)

func rowsToTableData(rows *sql.Rows) (*tableland.TableData, error) {
	columns, err := getColumnsData(rows)
	if err != nil {
		return nil, fmt.Errorf("get columns from rows: %s", err)
	}
	rowsData, err := getRowsData(rows, len(columns))
	if err != nil {
		return nil, err
	}

	return &tableland.TableData{
		Columns: columns,
		Rows:    rowsData,
	}, nil
}

func getColumnsData(rows *sql.Rows) ([]tableland.Column, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns from sql.Rows: %s", err)
	}
	columns := make([]tableland.Column, len(cols))
	for i := range cols {
		columns[i] = tableland.Column{Name: cols[i]}
	}
	return columns, nil
}

func getRowsData(rows *sql.Rows, numColumns int) ([][]*tableland.ColumnValue, error) {
	rowsData := make([][]*tableland.ColumnValue, 0)
	for rows.Next() {
		vals := make([]*tableland.ColumnValue, numColumns)
		for i := range vals {
			val := &tableland.ColumnValue{}
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
