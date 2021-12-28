package impl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/tests"
)

func TestRollback(t *testing.T) {
	url, err := tests.PostgresURL()
	require.NoError(t, err)

	ctx := context.Background()
	store, err := New(ctx, url, true)
	require.NoError(t, err)

	err = store.Begin(ctx) // begin transaction
	require.NoError(t, err)

	err = store.Write(ctx, "CREATE TABLE valid (a int);")
	require.NoError(t, err)

	_, err = store.Read(ctx, "SELECT * FROM valid") // SELECT inside a transaction before rollback (table exist)
	require.NoError(t, err)

	err = store.Rollback(ctx)
	require.NoError(t, err)

	_, err = store.Read(ctx, "SELECT * FROM valid") // Same select as above after rollback (table does not exist)
	require.Error(t, err)
}

func TestRollbackWithoutTransaction(t *testing.T) {
	url, err := tests.PostgresURL()
	require.NoError(t, err)

	ctx := context.Background()
	store, err := New(ctx, url, false)
	require.NoError(t, err)

	err = store.Begin(ctx) // begin transaction
	require.NoError(t, err)

	err = store.Write(ctx, "CREATE TABLE valid (a int);")
	require.NoError(t, err)

	_, err = store.Read(ctx, "SELECT * FROM valid") // SELECT inside a transaction before rollback (table exist)
	require.NoError(t, err)

	err = store.Rollback(ctx)
	require.NoError(t, err)

	_, err = store.Read(ctx, "SELECT * FROM valid") // Same select as above after rollback (table does exist)
	require.NoError(t, err)
}
