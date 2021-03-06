package db

import (
	"database/sql"
	"time"
)

type Registry struct {
	CreatedAt  time.Time
	ID         int64
	Structure  string
	Controller string
	Prefix     string
	ChainID    int64
}

type SystemAcl struct {
	TableID    int64
	Controller string
	Privileges int
	CreatedAt  time.Time
	UpdatedAt  *time.Time
	ChainID    int64
}

type SystemController struct {
	ChainID    int64
	TableID    string
	Controller string
}

type SystemPendingTx struct {
	ChainID        int64
	Address        string
	Hash           string
	Nonce          int64
	CreatedAt      time.Time
	BumpPriceCount int32
}

type SystemTxnProcessor struct {
	BlockNumber sql.NullInt64
	ChainID     int64
}

type SystemTxnReceipt struct {
	ChainID      int64
	BlockNumber  int64
	IndexInBlock int64
	TxnHash      string
	Error        sql.NullString
	TableID      sql.NullInt64
}
