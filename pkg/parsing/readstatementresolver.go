package parsing

import (
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sharedmemory"
)

// ReadStatementResolver implements the interface for custom functions resolution of read statements.
type ReadStatementResolver struct {
	sm *sharedmemory.SharedMemory
}

// NewReadStatementResolver creates a new ReadStatementResolver.
func NewReadStatementResolver(sm *sharedmemory.SharedMemory) *ReadStatementResolver {
	return &ReadStatementResolver{sm: sm}
}

// GetBlockNumber returns the block number for a given chain id.
func (rqr *ReadStatementResolver) GetBlockNumber(chainID int64) (int64, bool) {
	return rqr.sm.GetLastSeenBlockNumber(tableland.ChainID(chainID))
}
