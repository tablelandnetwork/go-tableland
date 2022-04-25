package sqlstore

import (
	"time"

	"github.com/textileio/go-tableland/internal/tableland"
)

// Table represents a system-wide table stored in Tableland.
type Table struct {
	ID         tableland.TableID `json:"id"`         // table id
	Controller string            `json:"controller"` // controller address
	Name       string            `json:"name"`
	Structure  string            `json:"structure"`
	CreatedAt  time.Time         `json:"created_at"`
}

// TableMetadata represents table metadata (OpenSea standard).
type TableMetadata struct {
	Name        string                   `json:"name"`
	ExternalURL string                   `json:"external_url"`
	Image       string                   `json:"image"`
	Attributes  []TableMetadataAttribute `json:"attributes"`
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
	TableID    tableland.TableID
	Privileges tableland.Privileges
	CreatedAt  time.Time
	UpdatedAt  *time.Time
}
