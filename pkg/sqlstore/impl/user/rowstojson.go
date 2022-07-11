package user

import (
	"database/sql"
	"fmt"

	"github.com/textileio/go-tableland/pkg/sqlstore"
)

func rowsToJSON(rows *sql.Rows) (*sqlstore.UserRows, error) {
	columns, err := getColumnsData(rows)
	if err != nil {
		return nil, fmt.Errorf("get columns from rows: %s", err)
	}
	rowsData, err := getRowsData(rows, len(columns))
	if err != nil {
		return nil, err
	}

	return &sqlstore.UserRows{
		Columns: columns,
		Rows:    rowsData,
	}, nil
}

func getColumnsData(rows *sql.Rows) ([]sqlstore.UserColumn, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns from sql.Rows: %s", err)
	}
	columns := make([]sqlstore.UserColumn, len(cols))
	for i := range cols {
		columns[i] = sqlstore.UserColumn{Name: cols[i]}
	}
	return columns, nil
}

func getRowsData(rows *sql.Rows, numColumns int) ([][]*sqlstore.UserValue, error) {
	rowsData := make([][]*sqlstore.UserValue, 0)
	for rows.Next() {
		vals := make([]*sqlstore.UserValue, numColumns)
		for i := range vals {
			val := &sqlstore.UserValue{}
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
