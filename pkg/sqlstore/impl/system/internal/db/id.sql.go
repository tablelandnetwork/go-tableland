package db

import (
	"context"
)

const (
	getId    = `SELECT id FROM system_id`
	insertId = `INSERT INTO system_id VALUES (?1)`
)

func (q *Queries) GetId(ctx context.Context) (string, error) {
	var id string
	if err := q.queryRow(ctx, q.getIdStmt, getId).Scan(&id); err != nil {
		return "", err
	}

	return id, nil
}

func (q *Queries) InsertId(ctx context.Context, id string) error {
	_, err := q.exec(ctx, q.insertIdStmt, insertId, id)
	return err
}
