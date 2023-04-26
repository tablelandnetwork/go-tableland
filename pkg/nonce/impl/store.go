package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/database"
	"github.com/textileio/go-tableland/pkg/database/db"
	"github.com/textileio/go-tableland/pkg/nonce"
)

// NonceStore relies on the SQLStore implementation for now.
type NonceStore struct {
	log      zerolog.Logger
	sqliteDB *database.SQLiteDB
}

// NewNonceStore creates a new nonce store.
func NewNonceStore(sqliteDB *database.SQLiteDB) nonce.NonceStore {
	log := sqliteDB.Log.With().
		Str("component", "noncestore").
		Logger()

	return &NonceStore{
		log:      log,
		sqliteDB: sqliteDB,
	}
}

// ListPendingTx lists all pendings txs.
func (s *NonceStore) ListPendingTx(
	ctx context.Context, chainID tableland.ChainID, addr common.Address,
) ([]nonce.PendingTx, error) {
	txs, err := s.sqliteDB.Queries.ListPendingTx(ctx, db.ListPendingTxParams{
		Address: addr.Hex(),
		ChainID: int64(chainID),
	})
	if err != nil {
		return []nonce.PendingTx{}, fmt.Errorf("nonce store list pending tx: %s", err)
	}

	pendingTxs := make([]nonce.PendingTx, 0)
	for _, r := range txs {
		tx := nonce.PendingTx{
			Address:        common.HexToAddress(r.Address),
			Nonce:          r.Nonce,
			Hash:           common.HexToHash(r.Hash),
			ChainID:        r.ChainID,
			BumpPriceCount: r.BumpPriceCount,
			CreatedAt:      time.Unix(r.CreatedAt, 0),
		}

		pendingTxs = append(pendingTxs, tx)
	}

	return pendingTxs, nil
}

// InsertPendingTx insert a new pending tx.
func (s *NonceStore) InsertPendingTx(
	ctx context.Context,
	chainID tableland.ChainID,
	addr common.Address,
	nonce int64, hash common.Hash,
) error {
	if err := s.sqliteDB.Queries.InsertPendingTx(ctx, db.InsertPendingTxParams{
		ChainID: int64(chainID),
		Address: addr.Hex(),
		Hash:    hash.Hex(),
		Nonce:   nonce,
	}); err != nil {
		return fmt.Errorf("nonce store insert pending tx: %s", err)
	}

	return nil
}

// DeletePendingTxByHash deletes a pending tx.
func (s *NonceStore) DeletePendingTxByHash(ctx context.Context, chainID tableland.ChainID, hash common.Hash) error {
	err := s.sqliteDB.Queries.DeletePendingTxByHash(ctx, db.DeletePendingTxByHashParams{
		ChainID: int64(chainID),
		Hash:    hash.Hex(),
	})
	if err != nil {
		return fmt.Errorf("nonce store delete pending tx: %s", err)
	}

	return nil
}

// ReplacePendingTxByHash replaces a pending tx hash with another and also bumps the counter
// to track how many times this happened for this nonce.
func (s *NonceStore) ReplacePendingTxByHash(
	ctx context.Context, chainID tableland.ChainID, oldHash common.Hash, newHash common.Hash,
) error {
	err := s.sqliteDB.Queries.ReplacePendingTxByHash(ctx, db.ReplacePendingTxByHashParams{
		Hash:    oldHash.Hex(),
		Hash_2:  newHash.Hex(),
		ChainID: int64(chainID),
	})
	if err != nil {
		return fmt.Errorf("replacing pending tx: %s", err)
	}

	return nil
}
