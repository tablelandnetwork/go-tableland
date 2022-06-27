package db

import (
	"context"
	"time"
)

const getTable = `-- name: GetTable :one
SELECT created_at, id, structure, controller, prefix, chain_id FROM registry WHERE chain_id =$1 AND id = $2
`

type GetTableParams struct {
	ChainID int64
	ID      string
}

func (q *Queries) GetTable(ctx context.Context, arg GetTableParams) (Registry, error) {
	row := q.queryRow(ctx, q.getTableStmt, getTable, arg.ChainID, arg.ID)
	var i Registry

	var createdAtEpoch int64
	err := row.Scan(
		&createdAtEpoch,
		&i.ID,
		&i.Structure,
		&i.Controller,
		&i.Prefix,
		&i.ChainID,
	)
	i.CreatedAt = time.Unix(createdAtEpoch, 0)
	return i, err
}

const getTablesByController = `-- name: GetTablesByController :many
SELECT created_at, id, structure, controller, prefix, chain_id FROM registry WHERE chain_id=?1 AND upper(controller) LIKE upper(?2)
`

type GetTablesByControllerParams struct {
	ChainID    int64
	Controller string
}

func (q *Queries) GetTablesByController(ctx context.Context, arg GetTablesByControllerParams) ([]Registry, error) {
	rows, err := q.query(ctx, q.getTablesByControllerStmt, getTablesByController, arg.ChainID, arg.Controller)
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
			&i.Prefix,
			&i.ChainID,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
