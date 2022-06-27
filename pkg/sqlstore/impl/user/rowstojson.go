package user

import (
	"database/sql"
	"fmt"
	"reflect"

	"github.com/textileio/go-tableland/pkg/sqlstore"
)

func rowsToJSON(rows *sql.Rows) (interface{}, error) {
	columnsData, err := getColumnsData(rows)
	if err != nil {
		return nil, fmt.Errorf("get columns from rows: %s", err)
	}
	rowsData, err := getRowsData(rows)
	if err != nil {
		return nil, err
	}

	return sqlstore.UserRows{
		Columns: columnsData,
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

func getRowsData(rows *sql.Rows) ([][]interface{}, error) {
	rowsData := make([][]interface{}, 0)
	for rows.Next() {
		colTypes, err := rows.ColumnTypes()
		if err != nil {
			return nil, fmt.Errorf("get column types from sql.Rows: %s", err)
		}
		scanArgs, err := getScanArgs(colTypes)
		if err != nil {
			return nil, err
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("scan row column: %s", err)
		}
		rowData := make([]interface{}, len(scanArgs))
		for i := 0; i < len(scanArgs); i++ {
			rowData[i], err = getValueFromScanArg(scanArgs[i])
			if err != nil {
				return nil, fmt.Errorf("get value from scan: %s", err)
			}
		}

		rowsData = append(rowsData, rowData)
	}

	return rowsData, nil
}

// do necessary conversions according to the type.
func getValueFromScanArg(arg interface{}) (interface{}, error) {
	switch arg := arg.(type) {
	case *sql.NullBool:
		if !arg.Valid {
			return nil, nil
		}
		return arg.Bool, nil
	case *sql.NullByte:
		if !arg.Valid {
			return nil, nil
		}
		return arg.Byte, nil
	case *sql.NullFloat64:
		if !arg.Valid {
			return nil, nil
		}
		return arg.Float64, nil
	case *sql.NullInt16:
		if !arg.Valid {
			return nil, nil
		}
		return arg.Int16, nil
	case *sql.NullInt32:
		if !arg.Valid {
			return nil, nil
		}
		return arg.Int32, nil
	case *sql.NullInt64:
		if !arg.Valid {
			return nil, nil
		}
		return arg.Int64, nil
	case *sql.NullString:
		if !arg.Valid {
			return nil, nil
		}
		return arg.String, nil
	case *sql.NullTime:
		if !arg.Valid {
			return nil, nil
		}
		return arg.Time, nil
	}
	return arg, nil
}

func getScanArgs(colTypes []*sql.ColumnType) ([]interface{}, error) {
	scanArgs := make([]interface{}, len(colTypes))
	for i := range colTypes {
		scanArgs[i] = reflect.New(colTypes[i].ScanType()).Interface()
	}

	return scanArgs, nil
}
