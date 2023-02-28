// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.15.0
// source: example.sql

package db

import (
	"context"
)

const getExampleId = `-- name: GetExampleId :one
SELECT id FROM example
`

func (q *Queries) GetExampleId(ctx context.Context) (string, error) {
	row := q.queryRow(ctx, q.getExampleIdStmt, getExampleId)
	var id string
	err := row.Scan(&id)
	return id, err
}

const insertExample = `-- name: InsertExample :exec
INSERT INTO example (id) VALUES (?)
`

func (q *Queries) InsertExample(ctx context.Context, id string) error {
	_, err := q.exec(ctx, q.insertExampleStmt, insertExample, id)
	return err
}