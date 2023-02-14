package chains

import (
	"context"

	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tables"
)

// ChainStack contains components running for a specific ChainID.
type ChainStack struct {
	Store          sqlstore.SystemStore
	Registry       tables.TablelandTables
	EventProcessor eventprocessor.EventProcessor
	// close gracefully closes all the chain stack components.
	Close func(ctx context.Context) error
}
