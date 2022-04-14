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
	systemStore sqlstore.SQLStore
}

// NewNonceStore creates a new nonce store.
func NewNonceStore(systemStore sqlstore.SQLStore) nonce.NonceStore {
	return &NonceStore{systemStore: systemStore}
}

// GetNonce returns the nonce stored in the database by a given address.
func (s *NonceStore) GetNonce(ctx context.Context, network nonce.Network, addr common.Address) (nonce.Nonce, error) {
	n, err := s.systemStore.GetNonce(ctx, string(network), addr)
	if err != nil {
		return nonce.Nonce{}, fmt.Errorf("nonce store get nonce: %s", err)
	}

	return n, nil
}

// UpsertNonce updates a nonce.
func (s *NonceStore) UpsertNonce(ctx context.Context, network nonce.Network, addr common.Address, nonce int64) error {
	err := s.systemStore.UpsertNonce(ctx, string(network), addr, nonce)
	if err != nil {
		return fmt.Errorf("nonce store upsert nonce: %s", err)
	}

	return nil
}

// ListPendingTx lists all pendings txs.
func (s *NonceStore) ListPendingTx(
	ctx context.Context,
	network nonce.Network,
	addr common.Address) ([]nonce.PendingTx, error) {
	txs, err := s.systemStore.ListPendingTx(ctx, string(network), addr)
	if err != nil {
		return []nonce.PendingTx{}, fmt.Errorf("nonce store list pending tx: %s", err)
	}

	return txs, nil
}

// InsertPendingTxAndUpsertNonce insert a new pending tx.
func (s *NonceStore) InsertPendingTxAndUpsertNonce(
	ctx context.Context,
	network nonce.Network,
	addr common.Address,
	nonce int64, hash common.Hash) error {
	tx, err := s.systemStore.Begin(ctx)
	if err != nil {
		return fmt.Errorf("opening tx: %s", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.systemStore.WithTx(tx).UpsertNonce(ctx, string(network), addr, nonce); err != nil {
		return fmt.Errorf("nonce store upsert nonce: %s", err)
	}

	if err := s.systemStore.WithTx(tx).InsertPendingTx(ctx, string(network), addr, nonce, hash); err != nil {
		return fmt.Errorf("nonce store insert pending tx: %s", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing changes: %s", err)
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
