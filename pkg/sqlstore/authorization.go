package sqlstore

import (
	"time"
)

// IsAuthorizedResult specifies whether or no an address is authorized.
type IsAuthorizedResult struct {
	IsAuthorized bool `json:"is_authorized"`
}

// AuthorizationRecord represents an authorized address.
type AuthorizationRecord struct {
	Address          string     `json:"address"`
	CreatedAt        time.Time  `json:"created_at"`
	LastSeen         *time.Time `json:"last_seen"`
	CreateTableCount int32      `json:"create_table_count"`
	RunSQLCount      int32      `json:"run_sql_count"`
}
