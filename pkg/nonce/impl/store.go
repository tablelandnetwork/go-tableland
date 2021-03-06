package impl

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

// NonceStore relies on the SQLStore implementation for now.
type NonceStore struct {
	systemStore sqlstore.SystemStore
}

// NewNonceStore creates a new nonce store.
func NewNonceStore(systemStore sqlstore.SystemStore) nonce.NonceStore {
	return &NonceStore{systemStore: systemStore}
}

// ListPendingTx lists all pendings txs.
func (s *NonceStore) ListPendingTx(
	ctx context.Context,
	addr common.Address,
) ([]nonce.PendingTx, error) {
	txs, err := s.systemStore.ListPendingTx(ctx, addr)
	if err != nil {
		return []nonce.PendingTx{}, fmt.Errorf("nonce store list pending tx: %s", err)
	}

	return txs, nil
}

// InsertPendingTx insert a new pending tx.
func (s *NonceStore) InsertPendingTx(
	ctx context.Context,
	addr common.Address,
	nonce int64, hash common.Hash,
) error {
	if err := s.systemStore.InsertPendingTx(ctx, addr, nonce, hash); err != nil {
		return fmt.Errorf("nonce store insert pending tx: %s", err)
	}

	return nil
}

// DeletePendingTxByHash deletes a pending tx.
func (s *NonceStore) DeletePendingTxByHash(ctx context.Context, hash common.Hash) error {
	err := s.systemStore.DeletePendingTxByHash(ctx, hash)
	if err != nil {
		return fmt.Errorf("nonce store delete pending tx: %s", err)
	}

	return nil
}

// ReplacePendingTxByHash replaces a pending tx hash with another and also bumps the counter
// to track how many times this happened for this nonce.
func (s *NonceStore) ReplacePendingTxByHash(ctx context.Context, oldHash common.Hash, newHash common.Hash) error {
	err := s.systemStore.ReplacePendingTxByHash(ctx, oldHash, newHash)
	if err != nil {
		return fmt.Errorf("replacing pending tx: %s", err)
	}

	return nil
}
