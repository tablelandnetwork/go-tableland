package impl

import (
	"context"
	"fmt"
	"math/big"

	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"

	"github.com/textileio/go-tableland/pkg/merkletree/publisher"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system/db"
)

// LeavesStore responsible for interacting with system_tree_leaves table.
type LeavesStore struct {
	log         zerolog.Logger
	systemStore sqlstore.SystemStore
}

// NewLeavesStore returns a new LeavesStore backed by database/sql.
func NewLeavesStore(systemStore sqlstore.SystemStore) *LeavesStore {
	log := logger.With().
		Str("component", "leavesstore").
		Logger()

	leavesstore := &LeavesStore{
		log:         log,
		systemStore: systemStore,
	}

	return leavesstore
}

// FetchLeavesByChainIDAndBlockNumber fetches chain ids and block numbers to be processed.
func (s *LeavesStore) FetchLeavesByChainIDAndBlockNumber(
	ctx context.Context,
	chainID int64,
	blockNumber int64,
) ([]publisher.TreeLeaves, error) {
	params := db.FetchLeavesByChainIDAndBlockNumberParams{
		ChainID:     chainID,
		BlockNumber: blockNumber,
	}
	rows, err := s.systemStore.Queries().FetchLeavesByChainIDAndBlockNumber(ctx, params)
	if err != nil {
		return []publisher.TreeLeaves{}, fmt.Errorf("fetching leaves by chain id and block number: %s", err)
	}

	leaves := make([]publisher.TreeLeaves, len(rows))
	for i, row := range rows {
		leaves[i] = publisher.TreeLeaves{
			ChainID:     row.ChainID,
			BlockNumber: row.BlockNumber,
			TableID:     big.NewInt(row.TableID),
			TablePrefix: row.Prefix,
			Leaves:      row.Leaves,
		}
	}

	return leaves, nil
}

// FetchChainIDAndBlockNumber fetches rows from leaves store by chain id and block number.
func (s *LeavesStore) FetchChainIDAndBlockNumber(ctx context.Context) ([]publisher.ChainIDBlockNumberPair, error) {
	rows, err := s.systemStore.Queries().FetchChainIDAndBlockNumber(ctx)
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
	if err := s.systemStore.Queries().DeleteProcessing(ctx, db.DeleteProcessingParams{
		ChainID:     chainID,
		BlockNumber: blockNumber,
	}); err != nil {
		return fmt.Errorf("delete processing: %s", err)
	}

	return nil
}
