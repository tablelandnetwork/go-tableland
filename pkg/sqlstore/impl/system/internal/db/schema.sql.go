package db

import (
	"context"
)

const getSchemaByTableName = `SELECT sql FROM sqlite_master WHERE name=?1;`

type GetSchemaByTableNameParams struct {
	TableName string
}

func (q *Queries) GetSchemaByTableName(ctx context.Context, arg GetSchemaByTableNameParams) (string, error) {
	row := q.queryRow(ctx, q.getReceiptStmt, getSchemaByTableName, arg.TableName)
	var createTableStatement string
	err := row.Scan(
		&createTableStatement,
	)
	return createTableStatement, err
}
