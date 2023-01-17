package readstatementresolver

import (
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
)

// ReadStatementResolver implements the interface for custom functions resolution of read statements.
type ReadStatementResolver struct {
	leb map[tableland.ChainID]func() int64
}

// New creates a new ReadStatementResolver.
func New(chainStacks map[tableland.ChainID]eventprocessor.EventProcessor) *ReadStatementResolver {
	leb := make(map[tableland.ChainID]func() int64, len(chainStacks))
	for chainID, ep := range chainStacks {
		leb[chainID] = ep.GetLastExecutedBlockNumber
	}
	return &ReadStatementResolver{leb: leb}
}

// GetBlockNumber returns the block number for a given chain id.
func (rqr *ReadStatementResolver) GetBlockNumber(chainID int64) (int64, bool) {
	r, ok := rqr.leb[tableland.ChainID(chainID)]
	if !ok {
		return 0, false
	}

	return r(), true
}
