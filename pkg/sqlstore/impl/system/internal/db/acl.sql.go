package db

import (
	"context"
	"database/sql"
	"time"
)

const getAclByTableAndController = `
SELECT table_id, controller, privileges, created_at, updated_at, chain_id FROM system_acl WHERE chain_id = ?3 AND table_id = ?2 AND upper(controller) LIKE upper(?1)
`

type GetAclByTableAndControllerParams struct {
	Controller string
	TableID    int64
	ChainID    int64
}

func (q *Queries) GetAclByTableAndController(ctx context.Context, arg GetAclByTableAndControllerParams) (SystemAcl, error) {
	row := q.queryRow(ctx, q.getAclByTableAndControllerStmt, getAclByTableAndController, arg.Controller, arg.TableID, arg.ChainID)
	var i SystemAcl
	var createdAtUnix int64
	var updatedAtUnix sql.NullInt64
	err := row.Scan(
		&i.TableID,
		&i.Controller,
		&i.Privileges,
		&createdAtUnix,
		&updatedAtUnix,
		&i.ChainID,
	)
	i.CreatedAt = time.Unix(createdAtUnix, 0)
	if updatedAtUnix.Valid {
		updatedAt := time.Unix(updatedAtUnix.Int64, 0)
		i.UpdatedAt = &updatedAt
	}
	return i, err
}
