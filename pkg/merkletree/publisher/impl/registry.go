package impl

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

// MerkleRootRegistryLogger is an implementat that simply logs the roots.
type MerkleRootRegistryLogger struct {
	logger zerolog.Logger
}

// NewMerkleRootRegistryLogger creates a new MerkleRootRegistryLogger.
func NewMerkleRootRegistryLogger(logger zerolog.Logger) *MerkleRootRegistryLogger {
	return &MerkleRootRegistryLogger{logger: logger}
}

// Publish logs the roots.
func (r *MerkleRootRegistryLogger) Publish(
	_ context.Context,
	chainID int64,
	blockNumber int64,
	tables []int64,
	roots [][]byte,
) error {
	l := r.logger.Info().
		Int64("chain_id", chainID).
		Int64("block_number", blockNumber).
		Ints64("tables", tables)

	for i, root := range roots {
		l.Hex(fmt.Sprintf("root_%d", tables[i]), root)
	}

	l.Msg("merkle roots")

	return nil
}
