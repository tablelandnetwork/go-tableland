package chains

import (
	"context"

	"github.com/textileio/go-tableland/pkg/eventprocessor"
)

// ChainStack contains components running for a specific ChainID.
type ChainStack struct {
	EventProcessor eventprocessor.EventProcessor
	// close gracefully closes all the chain stack components.
	Close func(ctx context.Context) error
}
