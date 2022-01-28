package user

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/tests"
)

func TestSingleQuery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bus, pool := newBUSWithTable(t)

	err := bus.OpenBatch(ctx)
	require.NoError(t, err)

	wq1 := `insert into foo values ('one')`
	err = bus.Exec(ctx, []string{wq1})
	require.NoError(t, err)

	require.NoError(t, bus.CloseBatch(ctx))
	require.NoError(t, bus.Close(ctx))

	require.Equal(t, 1, rowCount(t, pool))
}

func TestMultipleQueries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bus, pool := newBUSWithTable(t)

	err := bus.OpenBatch(ctx)
	require.NoError(t, err)

	{
		wq1 := `insert into foo values ('one')`
		err = bus.Exec(ctx, []string{wq1})
		require.NoError(t, err)
	}
	{
		wq1 := `insert into foo values ('two')`
		wq2 := `insert into foo values ('three')`
		err = bus.Exec(ctx, []string{wq1, wq2})
		require.NoError(t, err)
	}
	{
		wq1 := `insert into foo values ('four')`
		err = bus.Exec(ctx, []string{wq1})
		require.NoError(t, err)
	}

	require.NoError(t, bus.CloseBatch(ctx))
	require.NoError(t, bus.Close(ctx))

	require.Equal(t, 4, rowCount(t, pool))
}

func TestMultipleWithSingleFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bus, pool := newBUSWithTable(t)

	err := bus.OpenBatch(ctx)
	require.NoError(t, err)

	{
		wq1_1 := `insert into foo values ('one')`
		err = bus.Exec(ctx, []string{wq1_1})
		require.NoError(t, err)
	}
	{
		wq2_1 := `insert into foo values ('two')`
		wq2_2 := `insert into foo_wrong_table_name values ('three')`
		err = bus.Exec(ctx, []string{wq2_1, wq2_2})
		require.Error(t, err)
	}
	{
		wq3_1 := `insert into foo values ('four')`
		err = bus.Exec(ctx, []string{wq3_1})
		require.NoError(t, err)
	}

	require.NoError(t, bus.CloseBatch(ctx))
	require.NoError(t, bus.Close(ctx))

	// We executed a single batch, with 3 Exec calls.
	// The second Exec should have failed as a whole.
	//
	// Note that its wq2_1 succeded, but wq2_2 failed, this means:
	// 1. wq1_1 and wq3_1 should survive the whole batch commit.
	// 2. despite wq2_1 apparently should succeed, wq2_2 failure should rollback
	//    both wq2_* statements.
	require.Equal(t, 2, rowCount(t, pool))
}

func TestWithAbruptClose(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bus, pool := newBUSWithTable(t)

	err := bus.OpenBatch(ctx)
	require.NoError(t, err)

	{
		wq1_1 := `insert into foo values ('one')`
		err = bus.Exec(ctx, []string{wq1_1})
		require.NoError(t, err)
	}
	{
		wq2_1 := `insert into foo values ('two')`
		wq2_2 := `insert into foo values ('three')`
		err = bus.Exec(ctx, []string{wq2_1, wq2_2})
		require.NoError(t, err)
	}

	require.NoError(t, bus.Close(ctx))

	// The opened batch wasn't bus.CloseBatch(), but we simply
	// closed the whole store. This should rollback any ongoing
	// opened batch and leave db state correctly.
	require.Equal(t, 0, rowCount(t, pool))

}

func rowCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()

	q := "select count(*) from foo"
	row := pool.QueryRow(context.Background(), q)
	var rowCount int
	err := row.Scan(&rowCount)
	require.NoError(t, err)

	return rowCount
}

func newBUSWithTable(t *testing.T) (*BatchedUserStore, *pgxpool.Pool) {
	t.Helper()

	url, err := tests.PostgresURL()
	require.NoError(t, err)

	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	require.NoError(t, err)

	createTableQuery := "create table foo (name text)"
	_, err = pool.Exec(ctx, createTableQuery)
	require.NoError(t, err)

	return NewBatchedUserStore(pool), pool
}
