package chains

import (
	"context"

	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tableregistry"
)

// ChainStack contains components running for a specific ChainID.
type ChainStack struct {
	Store    sqlstore.SystemStore
	Registry tableregistry.TableRegistry

	// close gracefully closes all the chain stack components.
	Close func(ctx context.Context) error
}
