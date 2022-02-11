package user

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/textileio/go-tableland/pkg/parsing"
)

func rowsToJSON(rows pgx.Rows) (interface{}, error) {
	fields := rows.FieldDescriptions()

	columnsData := getColumnsData(fields)
	rowsData, err := getRowsData(rows, fields, len(columnsData))
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"columns": columnsData,
		"rows":    rowsData,
	}, nil
}

func getColumnsData(fields []pgproto3.FieldDescription) []struct {
	Name string `json:"name"`
} {
	columns := make([]struct {
		Name string `json:"name"`
	}, 0)
	for _, col := range fields {
		columns = append(columns, struct {
			Name string `json:"name"`
		}{string(col.Name)})
	}

	return columns
}

func getRowsData(rows pgx.Rows, fields []pgproto3.FieldDescription, nColumns int) ([][]interface{}, error) {
	rowsData := make([][]interface{}, 0)
	for rows.Next() {
		scanArgs, err := getScanArgs(fields, nColumns)
		if err != nil {
			return nil, err
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("scan row column: %s", err)
		}
		rowData := make([]interface{}, nColumns)
		for i := 0; i < nColumns; i++ {
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
	}

	return arg, nil
}

func getScanArgs(fields []pgproto3.FieldDescription, nColumns int) ([]interface{}, error) {
	scanArgs := make([]interface{}, nColumns)
	for i := 0; i < nColumns; i++ {
		t, err := getTypeFromOID(fields[i].DataTypeOID)
		if err != nil {
			return nil, err
		}
		scanArgs[i] = t
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
