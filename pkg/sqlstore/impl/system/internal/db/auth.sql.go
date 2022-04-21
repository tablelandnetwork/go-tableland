// Code generated by sqlc. DO NOT EDIT.
// source: auth.sql

package db

import (
	"context"
)

const incrementCreateTableCount = `-- name: IncrementCreateTableCount :exec
UPDATE system_auth SET create_table_count = create_table_count+1, last_seen = NOW() WHERE address ILIKE $1
`

func (q *Queries) IncrementCreateTableCount(ctx context.Context, address string) error {
	_, err := q.db.Exec(ctx, incrementCreateTableCount, address)
	return err
}

const incrementRunSQLCount = `-- name: IncrementRunSQLCount :exec
UPDATE system_auth SET run_sql_count = run_sql_count+1, last_seen = NOW() WHERE address ILIKE $1
`

func (q *Queries) IncrementRunSQLCount(ctx context.Context, address string) error {
	_, err := q.db.Exec(ctx, incrementRunSQLCount, address)
	return err
}
