package publisher

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math/big"
	"sync"
	"time"

	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/merkletree"
)

// TreeLeaves represents a leaves' snapshot of a table at a particular block.
type TreeLeaves struct {
	Leaves      []byte
	ChainID     int64
	TableID     *big.Int
	BlockNumber int64
	TablePrefix string
}

// ChainIDBlockNumberPair is a pair of ChainID and BlockNumber.
type ChainIDBlockNumberPair struct {
	ChainID     int64
	BlockNumber int64
}

// LeavesStore defines the API for fetching leaves from trees that need to be built.
type LeavesStore interface {
	FetchLeavesByChainIDAndBlockNumber(context.Context, int64, int64) ([]TreeLeaves, error)
	FetchChainIDAndBlockNumber(context.Context) ([]ChainIDBlockNumberPair, error)
	DeleteProcessing(context.Context, int64, int64) error
}

// MerkleTreeStore defines the API for storing the merkle tree.
type MerkleTreeStore interface {
	Store(chainID int64, tableID *big.Int, blockNumber int64, tree *merkletree.MerkleTree) error
}

// MerkleRootRegistry defines the API for publishing root.
type MerkleRootRegistry interface {
	// Publish publishes the roots of multiple tables at a particular block.
	Publish(ctx context.Context, chainID int64, blockNumber int64, tables []*big.Int, roots [][]byte) error
}

// MerkleRootPublisher is responsible for building Merkle Tree and publishing the root.
type MerkleRootPublisher struct {
	leavesStore LeavesStore        // where leaves are stored
	treeStore   MerkleTreeStore    // where trees are stored
	registry    MerkleRootRegistry // where root will be published

	// wallet   *wallet.Wallet
	interval time.Duration

	quitOnce sync.Once
	quit     chan struct{}
}

// NewMerkleRootPublisher creates a new publisher.
func NewMerkleRootPublisher(
	leavesStore LeavesStore,
	treeStore MerkleTreeStore,
	registry MerkleRootRegistry,
	interval time.Duration,
) *MerkleRootPublisher {
	return &MerkleRootPublisher{
		leavesStore: leavesStore,
		treeStore:   treeStore,
		registry:    registry,

		// wallet:   wallet,
		interval: interval,
		quit:     make(chan struct{}),
	}
}

var log = logger.With().
	Str("component", "merkletreepublisher").
	Logger()

// Start starts the publisher.
func (p *MerkleRootPublisher) Start() {
	ctx := context.Background()

	ticker := time.NewTicker(p.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := p.publish(ctx); err != nil {
					log.Err(err).Msg("failed to publish merkle root")
				}
			case <-p.quit:
				log.Info().Msg("quiting merkle root publisher")
				ticker.Stop()
				return
			}
		}
	}()
}

// Close closes the published goroutine.
func (p *MerkleRootPublisher) Close() {
	p.quitOnce.Do(func() {
		p.quit <- struct{}{}
		close(p.quit)
	})
}

func (p *MerkleRootPublisher) publish(ctx context.Context) error {
	chainIDBlockNumberPairs, err := p.leavesStore.FetchChainIDAndBlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("fetching block number and chain id pairs: %s", err)
	}

	// we do `n` publish calls, where n is the number of chains
	for _, pair := range chainIDBlockNumberPairs {
		tableLeaves, err := p.leavesStore.FetchLeavesByChainIDAndBlockNumber(ctx, pair.ChainID, pair.BlockNumber)
		if err != nil {
			return fmt.Errorf("fetch unpublished metrics: %s", err)
		}

		if len(tableLeaves) == 0 {
			return nil
		}

		tableIDs, roots := make([]*big.Int, len(tableLeaves)), make([][]byte, len(tableLeaves))
		for i, table := range tableLeaves {
			if table.ChainID != pair.ChainID {
				return fmt.Errorf("chain id mismatch (%d, %d)", table.ChainID, pair.ChainID)
			}

			if table.BlockNumber != pair.BlockNumber {
				return fmt.Errorf("block number mismatch (%d, %d)", table.BlockNumber, pair.BlockNumber)
			}

			// gotta use the block size of the hash used to encode the leaves
			chunks, err := chunker(table.Leaves, fnv.New128a().Size())
			if err != nil {
				return fmt.Errorf("breaking leaves into chunks: %s", err)
			}

			tree, err := merkletree.NewTree(chunks, nil)
			if err != nil {
				return fmt.Errorf("building a tree: %s", err)
			}

			tableIDs[i], roots[i] = table.TableID, tree.MerkleRoot()
			if err := p.treeStore.Store(pair.ChainID, table.TableID, pair.BlockNumber, tree); err != nil {
				return fmt.Errorf("storing the tree chain: %d, table: %d): %s", pair.ChainID, table.TableID.Int64(), err)
			}
		}

		if err := p.registry.Publish(ctx, pair.ChainID, pair.BlockNumber, tableIDs, roots); err != nil {
			return fmt.Errorf("publishing root: %s", err)
		}

		if err := p.leavesStore.DeleteProcessing(ctx, pair.ChainID, pair.BlockNumber); err != nil {
			return fmt.Errorf("delete processing: %s", err)
		}
	}

	return nil
}

func chunker(data []byte, size int) ([][]byte, error) {
	if len(data)%size != 0 {
		return [][]byte{}, errors.New("data length should be multiple of size")
	}
	chunks := make([][]byte, len(data)/size)
	for i := 0; i < len(data); i += size {
		chunks[i/size] = data[i : i+size]
	}

	return chunks, nil
}
