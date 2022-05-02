package chains

import (
	"context"

	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tableregistry"
)

// ChainStack contains components running for a specific ChainID.
type ChainStack struct {
	Store    sqlstore.SQLStore
	Registry tableregistry.TableRegistry
	Parser   parsing.SQLValidator

	// close gracefully closes all the chain stack components.
	Close func(ctx context.Context) error
}
