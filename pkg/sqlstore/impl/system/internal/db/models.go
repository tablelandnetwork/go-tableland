// Code generated by sqlc. DO NOT EDIT.

package db

import (
	"database/sql"
	"time"

	"github.com/jackc/pgtype"
)

type Registry struct {
	CreatedAt   time.Time
	ID          pgtype.Numeric
	Structure   string
	Controller  string
	Description string
	Name        string
}

type SystemAcl struct {
	TableID    pgtype.Numeric
	Controller string
	Privileges []string
	CreatedAt  time.Time
	UpdatedAt  sql.NullTime
}

type SystemAuth struct {
	Address          string
	CreatedAt        time.Time
	LastSeen         sql.NullTime
	CreateTableCount int32
	RunSqlCount      int32
}

type SystemTxnProcessor struct {
	BlockNumber sql.NullInt64
}
