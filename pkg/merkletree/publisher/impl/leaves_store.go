package impl

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/textileio/go-tableland/pkg/merkletree/publisher"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system/db"
)

// LeavesStore responsible for interacting with system_tree_leaves table.
type LeavesStore struct {
	log zerolog.Logger
	db  *impl.SQLiteDB
}

// NewLeavesStore returns a new LeavesStore backed by database/sql.
func NewLeavesStore(sqlite *impl.SQLiteDB) *LeavesStore {
	log := sqlite.Log.With().
		Str("component", "leavesstore").
		Logger()

	leavesstore := &LeavesStore{
		log: log,
		db:  sqlite,
	}

	return leavesstore
}

// FetchLeavesByChainIDAndBlockNumber fetches chain ids and block numbers to be processed.
func (s *LeavesStore) FetchLeavesByChainIDAndBlockNumber(
	ctx context.Context,
	chainID int64,
	blockNumber int64,
) ([]publisher.TreeLeaves, error) {
	rows, err := s.db.Queries.FetchLeavesByChainIDAndBlockNumber(ctx, db.FetchLeavesByChainIDAndBlockNumberParams{
		ChainID:     chainID,
		BlockNumber: blockNumber,
	})
	if err != nil {
		return []publisher.TreeLeaves{}, fmt.Errorf("fetching leaves by chain id and block number: %s", err)
	}

	leaves := make([]publisher.TreeLeaves, len(rows))
	for i, row := range rows {
		leaves[i] = publisher.TreeLeaves{
			ChainID:     row.ChainID,
			BlockNumber: row.BlockNumber,
			TableID:     row.TableID,
			TablePrefix: row.Prefix,
			Leaves:      row.Leaves,
		}
	}

	return leaves, nil
}

// FetchChainIDAndBlockNumber fetches rows from leaves store by chain id and block number.
func (s *LeavesStore) FetchChainIDAndBlockNumber(ctx context.Context) ([]publisher.ChainIDBlockNumberPair, error) {
	rows, err := s.db.Queries.FetchChainIDAndBlockNumber(ctx)
	if err != nil {
		return []publisher.ChainIDBlockNumberPair{}, fmt.Errorf("fetching chain id and block number: %s", err)
	}

	pairs := make([]publisher.ChainIDBlockNumberPair, len(rows))
	for i, row := range rows {
		pairs[i] = publisher.ChainIDBlockNumberPair{
			ChainID:     row.ChainID,
			BlockNumber: row.BlockNumber,
		}
	}

	return pairs, nil
}

// DeleteProcessing deletes rows that are marked as processing.
func (s *LeavesStore) DeleteProcessing(ctx context.Context, chainID int64, blockNumber int64) error {
	if err := s.db.Queries.DeleteProcessing(ctx, db.DeleteProcessingParams{
		ChainID:     chainID,
		BlockNumber: blockNumber,
	}); err != nil {
		return fmt.Errorf("delete processing: %s", err)
	}

	return nil
}
