package db

import (
	"context"
	"time"
)

const getTable = `
SELECT created_at, id, structure, controller, prefix, chain_id FROM registry WHERE chain_id =?1 AND id = ?2
`

type GetTableParams struct {
	ChainID int64
	ID      int64
}

func (q *Queries) GetTable(ctx context.Context, arg GetTableParams) (Registry, error) {
	row := q.queryRow(ctx, q.getTableStmt, getTable, arg.ChainID, arg.ID)
	var i Registry
	var createdAtUnix int64
	err := row.Scan(
		&createdAtUnix,
		&i.ID,
		&i.Structure,
		&i.Controller,
		&i.Prefix,
		&i.ChainID,
	)
	i.CreatedAt = time.Unix(createdAtUnix, 0)
	return i, err
}

const getTablesByController = `
SELECT id, structure, controller, prefix, chain_id FROM registry WHERE chain_id=?1 AND upper(controller) LIKE upper(?2)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getTablesByStructure = `
SELECT id, structure, controller, prefix, chain_id FROM registry WHERE chain_id=?1 AND structure=?2
`

type GetTablesByStructureParams struct {
	ChainID   int64
	Structure string
}

func (q *Queries) GetTablesByStructure(ctx context.Context, arg GetTablesByStructureParams) ([]Registry, error) {
	rows, err := q.query(ctx, q.getTableStmt, getTablesByStructure, arg.ChainID, arg.Structure)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Registry
	for rows.Next() {
		var i Registry
		if err := rows.Scan(
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
