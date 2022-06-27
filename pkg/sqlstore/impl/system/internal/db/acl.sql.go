// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.13.0
// source: acl.sql

package db

import (
	"context"

	"github.com/lib/pq"
)

const getAclByTableAndController = `-- name: GetAclByTableAndController :one
SELECT table_id, controller, privileges, created_at, updated_at, chain_id FROM system_acl WHERE chain_id = $3 AND table_id = $2 AND controller ILIKE $1
`

type GetAclByTableAndControllerParams struct {
	Controller string
	TableID    string
	ChainID    int64
}

func (q *Queries) GetAclByTableAndController(ctx context.Context, arg GetAclByTableAndControllerParams) (SystemAcl, error) {
	row := q.queryRow(ctx, q.getAclByTableAndControllerStmt, getAclByTableAndController, arg.Controller, arg.TableID, arg.ChainID)
	var i SystemAcl
	err := row.Scan(
		&i.TableID,
		&i.Controller,
		pq.Array(&i.Privileges),
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ChainID,
	)
	return i, err
}
