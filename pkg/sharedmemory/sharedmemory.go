package sharedmemory

import (
	"sync"

	"github.com/textileio/go-tableland/internal/tableland"
)

// SharedMemory is a in-memory thread-safe data structure to exchange data between the validator and gateway.
type SharedMemory struct {
	mu                  sync.RWMutex
	lastSeenBlockNumber map[tableland.ChainID]int64
}

// NewSharedMemory creates new SharedMemory object.
func NewSharedMemory() *SharedMemory {
	return &SharedMemory{
		lastSeenBlockNumber: make(map[tableland.ChainID]int64),
	}
}

// SetLastSeenBlockNumber sets the last seen block number of a specific chain.
func (sm *SharedMemory) SetLastSeenBlockNumber(chainID tableland.ChainID, blockNumber int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastSeenBlockNumber[chainID] = blockNumber
}

// GetLastSeenBlockNumber get the last seen block number of a specific chain.
func (sm *SharedMemory) GetLastSeenBlockNumber(chainID tableland.ChainID) (int64, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	blockNumber, ok := sm.lastSeenBlockNumber[chainID]
	if !ok {
		return 0, false
	}
	return blockNumber, true
}
