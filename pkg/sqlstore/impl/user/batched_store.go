package user

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog/log"
)

var (
	errOnlyOneBatchCanExist = errors.New("only one batch can exist at the same time")
	errNoOpenedBatch        = errors.New("there isn't an open batch to exec the query")
)

type BatchedUserStore struct {
	pool *pgxpool.Pool

	lock       sync.Mutex
	currentTxn pgx.Tx
}

func NewBatchedUserStore(pool *pgxpool.Pool) *BatchedUserStore {
	return &BatchedUserStore{
		pool: pool,
	}
}

func (ab *BatchedUserStore) OpenBatch(ctx context.Context) error {
	ab.lock.Lock()
	defer ab.lock.Unlock()

	if ab.currentTxn != nil {
		return errOnlyOneBatchCanExist
	}

	ops := pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadWrite,
	}
	txn, err := ab.pool.BeginTx(ctx, ops)
	if err != nil {
		return fmt.Errorf("opening postgres transaction: %s", err)
	}
	ab.currentTxn = txn

	return nil
}

func (ab *BatchedUserStore) CloseBatch(ctx context.Context) error {
	ab.lock.Lock()
	defer ab.lock.Unlock()

	if ab.currentTxn == nil {
		return errNoOpenedBatch
	}
	defer func() {
		if err := ab.currentTxn.Rollback(ctx); err != nil {
			if err != pgx.ErrTxClosed {
				log.Error().Err(err).Msg("closing batch")
			}
		}
	}()

	if err := ab.currentTxn.Commit(ctx); err != nil {
		return fmt.Errorf("commit txn: %s", err)
	}
	return nil

}

// Exec executes a set of w-queries atomically inside an opened batch.
// If a batch wasn't opened, it returns an error.
//
// TODO(jsign): we should accept `[]MutatingQuery` instead of `[]string`,
// where MutatingQuery type would be created by the parser. This can give
// a bit more type-checking gurantee that we're receiving something appropiate.
func (ab *BatchedUserStore) Exec(ctx context.Context, wqueries []string) error {
	ab.lock.Lock()
	defer ab.lock.Unlock()
	if ab.currentTxn == nil {
		return errNoOpenedBatch
	}

	f := func(nestedTxn pgx.Tx) error {
		for _, wq := range wqueries {
			if _, err := nestedTxn.Exec(ctx, wq); err != nil {
				return fmt.Errorf("exec query: %s", err)
			}
		}

		return nil
	}
	if err := ab.currentTxn.BeginFunc(ctx, f); err != nil {
		return fmt.Errorf("running nested txn: %s", err)
	}

	return nil
}

func (ab *BatchedUserStore) Close(ctx context.Context) error {
	ab.lock.Lock()
	defer ab.lock.Unlock()

	if ab.currentTxn == nil {
		return nil
	}

	if err := ab.currentTxn.Rollback(ctx); err != nil {
		if err != pgx.ErrTxClosed {
			return fmt.Errorf("closing batch: %w", err)
		}
	}
	return nil
}
