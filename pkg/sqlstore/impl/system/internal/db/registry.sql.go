// Code generated by sqlc. DO NOT EDIT.
// source: registry.sql

package db

import (
	"context"

	"github.com/jackc/pgtype"
)

const getTable = `-- name: GetTable :one
SELECT created_at, id, structure, controller, name FROM registry WHERE id = $1
`

func (q *Queries) GetTable(ctx context.Context, id pgtype.Numeric) (Registry, error) {
	row := q.db.QueryRow(ctx, getTable, id)
	var i Registry
	err := row.Scan(
		&i.CreatedAt,
		&i.ID,
		&i.Structure,
		&i.Controller,
		&i.Name,
	)
	return i, err
}

const getTablesByController = `-- name: GetTablesByController :many
SELECT created_at, id, structure, controller, name FROM registry WHERE controller ILIKE $1
`

func (q *Queries) GetTablesByController(ctx context.Context, controller string) ([]Registry, error) {
	rows, err := q.db.Query(ctx, getTablesByController, controller)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Registry
	for rows.Next() {
		var i Registry
		if err := rows.Scan(
			&i.CreatedAt,
			&i.ID,
			&i.Structure,
			&i.Controller,
			&i.Name,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
