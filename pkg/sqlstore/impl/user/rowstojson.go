package user

import (
	"database/sql"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/jackc/pgtype"
	"github.com/textileio/go-tableland/pkg/parsing"
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
	if _, ok := (arg).(pgtype.Value); ok {
		if val, ok := (arg).(*pgtype.Numeric); ok {
			if val.Status == pgtype.Null {
				return nil, nil
			}
			br := &big.Rat{}
			if err := val.AssignTo(br); err != nil {
				return nil, fmt.Errorf("parsing numeric to bigrat: %s", err)
			}
			if br.IsInt() {
				return br.Num().String(), nil
			}
			return strings.TrimRight(br.FloatString(64), "0"), nil
		}

		if val, ok := (arg).(*pgtype.Date); ok {
			if val.Status == pgtype.Null {
				return nil, nil
			}

			buf := make([]byte, 0)
			buf, _ = val.EncodeText(pgtype.NewConnInfo(), buf)
			return string(buf), nil
		}

		if val, ok := (arg).(*pgtype.Timestamp); ok {
			if val.Status == pgtype.Null {
				return nil, nil
			}

			buf := make([]byte, 0)
			buf, _ = val.EncodeText(pgtype.NewConnInfo(), buf)
			return string(buf), nil
		}

		if val, ok := (arg).(*pgtype.Timestamptz); ok {
			if val.Status == pgtype.Null {
				return nil, nil
			}

			buf := make([]byte, 0)
			buf, _ = val.EncodeText(pgtype.NewConnInfo(), buf)
			return string(buf), nil
		}

		if val, ok := (arg).(*pgtype.UUID); ok {
			if val.Status == pgtype.Null {
				return nil, nil
			}

			buf := make([]byte, 0)
			buf, _ = val.EncodeText(pgtype.NewConnInfo(), buf)
			return string(buf), nil
		}

		if val, ok := (arg).(*pgtype.VarcharArray); ok {
			if val.Status == pgtype.Null {
				return nil, nil
			}

			buf := make([]byte, 0)
			buf, _ = val.EncodeText(pgtype.NewConnInfo(), buf)
			return string(buf), nil
		}
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

// given the column type OID find the corresponding Golang's type (it can be either a native or a custom type).
func getTypeFromOID(oid uint32) (interface{}, error) {
	at, ok := parsing.AcceptedTypes[oid]
	if !ok {
		return nil, fmt.Errorf("column type %d not supported", oid)
	}

	nt := reflect.New(reflect.TypeOf(at.GoType))
	return nt.Interface(), nil
}
