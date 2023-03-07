package impl

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/textileio/go-tableland/pkg/merkletree"
	"go.etcd.io/bbolt"
)

// MerkleTreeStore stores merkle trees.
type MerkleTreeStore struct {
	db *bbolt.DB
}

// NewMerkleTreeStore creates a new Merkle Tree store.
func NewMerkleTreeStore(path string) (*MerkleTreeStore, error) {
	db, err := bbolt.Open(path, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("opening database: %s", err)
	}

	return &MerkleTreeStore{
		db: db,
	}, nil
}

// Store stores a merkle tree.
func (s *MerkleTreeStore) Store(chainID int64, tableID *big.Int, _ int64, tree *merkletree.MerkleTree) error {
	tx, err := s.db.Begin(true)
	if err != nil {
		return fmt.Errorf("begin: %s", err)
	}

	bucket := make([]byte, 8)
	binary.LittleEndian.PutUint64(bucket, uint64(chainID))

	b, err := tx.CreateBucketIfNotExists(bucket)
	if err != nil {
		return fmt.Errorf("creating bucket: %s", err)
	}

	// bn := make([]byte, 8)
	// binary.LittleEndian.PutUint64(bn, uint64(blockNumber))

	// key := append(tableID.Bytes(), bn...)

	key := tableID.Bytes()
	if err := b.Put(key, tree.Marshal()); err != nil {
		return fmt.Errorf("storing the tree: %s", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %s", err)
	}

	return nil
}

// Get fetches a tree and deserialize it.
func (s *MerkleTreeStore) Get(chainID int64, tableID *big.Int, _ int64) (*merkletree.MerkleTree, error) {
	var tree *merkletree.MerkleTree
	if err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := make([]byte, 8)
		binary.LittleEndian.PutUint64(bucket, uint64(chainID))

		b := tx.Bucket(bucket)
		if b == nil {
			return fmt.Errorf("bucket is nil")
		}

		var err error
		tree, err = merkletree.Unmarshal(b.Get(tableID.Bytes()), nil)
		if err != nil {
			return fmt.Errorf("unmarshalling tree: %s", err)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("db view: %s", err)
	}

	return tree, nil
}

// Close closes the store.
func (s *MerkleTreeStore) Close() error {
	return s.db.Close()
}
