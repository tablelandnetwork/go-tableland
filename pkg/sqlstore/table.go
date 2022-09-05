package sqlstore

import (
	"fmt"
	"time"

	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/tables"
)

// Table represents a system-wide table stored in Tableland.
type Table struct {
	ID         tables.TableID    `json:"id"` // table id
	ChainID    tableland.ChainID `json:"chain_id"`
	Controller string            `json:"controller"` // controller address
	Prefix     string            `json:"prefix"`
	Structure  string            `json:"structure"`
	CreatedAt  time.Time         `json:"created_at"`
}

// Name returns table's full name.
func (t Table) Name() string {
	return fmt.Sprintf("%s_%d_%s", t.Prefix, t.ChainID, t.ID)
}

// TableSchema represents the schema of a table.
type TableSchema struct {
	Columns          []ColumnSchema
	TableConstraints []string
}

// ColumnSchema represents the schema of a column.
type ColumnSchema struct {
	Name        string
	Type        string
	Constraints []string
}

// TableMetadata represents table metadata (OpenSea standard).
type TableMetadata struct {
	Name        string                   `json:"name,omitempty"`
	ExternalURL string                   `json:"external_url"`
	Image       string                   `json:"image"`
	Message     string                   `json:"message,omitempty"`
	Attributes  []TableMetadataAttribute `json:"attributes,omitempty"`
}

// TableMetadataAttribute represents the table metadata attribute.
type TableMetadataAttribute struct {
	DisplayType string      `json:"display_type"`
	TraitType   string      `json:"trait_type"`
	Value       interface{} `json:"value"`
}

// SystemACL represents the system acl table.
type SystemACL struct {
	Controller string
	ChainID    tableland.ChainID
	TableID    tables.TableID
	Privileges tableland.Privileges
	CreatedAt  time.Time
	UpdatedAt  *time.Time
}
