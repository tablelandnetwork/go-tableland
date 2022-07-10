package user

import (
	"database/sql"
	"fmt"

	"github.com/textileio/go-tableland/pkg/sqlstore"
)

func rowsToJSON(rows *sql.Rows, jsonStrings bool) (*sqlstore.UserRows, error) {
	columns, err := getColumnsData(rows)
	if err != nil {
		return nil, fmt.Errorf("get columns from rows: %s", err)
	}
	rowsData, err := getRowsData(rows, len(columns), jsonStrings)
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

func getRowsData(rows *sql.Rows, numColumns int, jsonStrings bool) ([][]interface{}, error) {
	rowsData := make([][]interface{}, 0)
	for rows.Next() {
		scanArgs := make([]interface{}, numColumns)
		for i := range scanArgs {
			scanArgs[i] = &sqlstore.UserValue{JSONStrings: jsonStrings}
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("scan row column: %s", err)
		}
		rowsData = append(rowsData, scanArgs)
	}

	return rowsData, nil
}
