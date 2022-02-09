package system

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/tests"
)

func TestLastSeen(t *testing.T) {
	url, err := tests.PostgresURL()
	require.NoError(t, err)

	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	require.NoError(t, err)
	defer pool.Close()

	store, err := New(pool)
	require.NoError(t, err)

	err = store.Authorize(ctx, "address")
	require.NoError(t, err)

	rec, err := store.GetAuthorizationRecord(ctx, "address")
	require.NoError(t, err)
	require.Nil(t, rec.LastSeen)

	err = store.IncrementCreateTableCount(ctx, "address")
	require.NoError(t, err)

	err = store.IncrementRunSQLCount(ctx, "address")
	require.NoError(t, err)

	rec, err = store.GetAuthorizationRecord(ctx, "address")
	require.NoError(t, err)
	require.NotNil(t, rec.LastSeen)
	require.True(t, rec.LastSeen.After(time.Now().AddDate(0, 0, -1)))

	recs, err := store.ListAuthorized(ctx)
	require.NoError(t, err)
	require.Len(t, recs, 1)
	require.NotNil(t, recs[0].LastSeen)
	require.True(t, recs[0].LastSeen.After(time.Now().AddDate(0, 0, -1)))
}
