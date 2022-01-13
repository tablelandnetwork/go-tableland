package sqlstore

import (
	"time"

	"github.com/google/uuid"
)

// Table represents a system-wide table stored in Tableland.
type Table struct {
	UUID       uuid.UUID `json:"uuid"`       // table id
	Controller string    `json:"controller"` // controller address
	Type       string    `json:"type"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableMetadata represents table metadata (OpenSea standard).
type TableMetadata struct {
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
