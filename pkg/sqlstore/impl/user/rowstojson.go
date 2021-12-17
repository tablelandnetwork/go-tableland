package user

import (
	"fmt"

	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
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

		rows.Scan(scanArgs...)
		rowData := make([]interface{}, nColumns)
		for i := 0; i < nColumns; i++ {
			rowData[i] = getValueFromScanArg(scanArgs[i])
		}

		rowsData = append(rowsData, rowData)
	}

	return rowsData, nil
}

// do necessary conversions according to the type
func getValueFromScanArg(arg interface{}) interface{} {
	if val, ok := (arg).([]byte); ok {
		return string(val)
	}

	if _, ok := (arg).(pgtype.Value); ok {
		if val, ok := (arg).(*pgtype.Numeric); ok {
			if val.Status == pgtype.Null {
				return nil
			}

			buf := make([]byte, 0)
			buf, _ = val.EncodeText(pgtype.NewConnInfo(), buf)
			return string(buf)
		}

		if val, ok := (arg).(*pgtype.Timestamp); ok {
			if val.Status == pgtype.Null {
				return nil
			}

			buf := make([]byte, 0)
			buf, _ = val.EncodeText(pgtype.NewConnInfo(), buf)
			return string(buf)
		}

		if val, ok := (arg).(*pgtype.Timestamptz); ok {
			if val.Status == pgtype.Null {
				return nil
			}

			buf := make([]byte, 0)
			buf, _ = val.EncodeText(pgtype.NewConnInfo(), buf)
			return string(buf)
		}

		if val, ok := (arg).(*pgtype.UUID); ok {
			if val.Status == pgtype.Null {
				return nil
			}

			buf := make([]byte, 0)
			buf, _ = val.EncodeText(pgtype.NewConnInfo(), buf)
			return string(buf)
		}
	}

	return arg
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

// given the column type OID find the corresponding Golang's type (it can be either a native or a custom type)
// TODO: add more types
func getTypeFromOID(oid uint32) (t interface{}, err error) {
	switch oid {
	case pgtype.Int2OID, pgtype.Int4OID, pgtype.Int8OID:
		t = new(*int)
	case pgtype.TextOID, pgtype.VarcharOID, pgtype.BPCharOID, pgtype.DateOID:
		t = new(*string)
	case pgtype.BoolOID:
		t = new(*bool)
	case pgtype.Float4OID, pgtype.Float8OID:
		t = new(*float64)
	case pgtype.NumericOID:
		t = new(pgtype.Numeric)
	case pgtype.TimestampOID:
		t = new(pgtype.Timestamp)
	case pgtype.TimestamptzOID:
		t = new(pgtype.Timestamptz)
	case pgtype.UUIDOID:
		t = new(pgtype.UUID)
	default:
		err = fmt.Errorf("column type %d not supported", oid)
	}
	return t, err
}
