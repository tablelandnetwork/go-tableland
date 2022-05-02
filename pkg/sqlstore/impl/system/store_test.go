package system

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/tests"
)

func TestLastSeen(t *testing.T) {
	url := tests.PostgresURL(t)

	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	require.NoError(t, err)
	defer pool.Close()

	store, err := New(pool, 1337)
	require.NoError(t, err)

	err = store.IncrementCreateTableCount(ctx, "address")
	require.NoError(t, err)

	err = store.IncrementRunSQLCount(ctx, "address")
	require.NoError(t, err)
}
