package sqlstore

import (
	"time"

	"github.com/google/uuid"
)

// Table reprents a system-wide table stored in Tableland
type Table struct {
	UUID       uuid.UUID // table id
	Controller string    // controller address
	CreatedAt  time.Time
}
