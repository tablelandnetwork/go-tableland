package impl

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/tests"
)

func TestRunSQL(t *testing.T) {
	t.Parallel()
	t.Run("single-query", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		wq1 := `insert into foo values ('one')`
		err = b.ExecWriteQueries(ctx, []string{wq1})
		require.NoError(t, err)

		require.NoError(t, b.Commit(ctx))
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		require.Equal(t, 1, tableRowCount(t, pool, "foo"))
	})

	t.Run("multiple queries", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		{
			wq1 := `insert into foo values ('wq1one')`
			err = b.ExecWriteQueries(ctx, []string{wq1})
			require.NoError(t, err)
		}
		{
			wq1 := `insert into foo values ('wq1two')`
			wq2 := `insert into foo values ('wq2three')`
			err = b.ExecWriteQueries(ctx, []string{wq1, wq2})
			require.NoError(t, err)
		}
		{
			wq1 := `insert into foo values ('wq1four')`
			err = b.ExecWriteQueries(ctx, []string{wq1})
			require.NoError(t, err)
		}

		require.NoError(t, b.Commit(ctx))
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		require.Equal(t, 4, tableRowCount(t, pool, "foo"))
	})

	t.Run("multiple with signle failure", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		{
			wq1_1 := `insert into foo values ('onez')`
			err = b.ExecWriteQueries(ctx, []string{wq1_1})
			require.NoError(t, err)
		}
		{
			wq2_1 := `insert into foo values ('twoz')`
			wq2_2 := `insert into foo_wrong_table_name values ('threez')`
			err = b.ExecWriteQueries(ctx, []string{wq2_1, wq2_2})
			require.Error(t, err)
		}
		{
			wq3_1 := `insert into foo values ('fourz')`
			err = b.ExecWriteQueries(ctx, []string{wq3_1})
			require.NoError(t, err)
		}

		require.NoError(t, b.Commit(ctx))
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		// We executed a single batch, with 3 Exec calls.
		// The second Exec should have failed as a whole.
		//
		// Note that its wq2_1 succeeded, but wq2_2 failed, this means:
		// 1. wq1_1 and wq3_1 should survive the whole batch commit.
		// 2. despite wq2_1 apparently should succeed, wq2_2 failure should rollback
		//    both wq2_* statements.
		require.Equal(t, 2, tableRowCount(t, pool, "foo"))
	})

	t.Run("with abrupt close", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		{
			wq1_1 := `insert into foo values ('one')`
			err = b.ExecWriteQueries(ctx, []string{wq1_1})
			require.NoError(t, err)
		}
		{
			wq2_1 := `insert into foo values ('two')`
			wq2_2 := `insert into foo values ('three')`
			err = b.ExecWriteQueries(ctx, []string{wq2_1, wq2_2})
			require.NoError(t, err)
		}

		// Note: we don't do a Commit() call, thus all should be rollbacked.
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		// The opened batch wasn't txnp.CloseBatch(), but we simply
		// closed the whole store. This should rollback any ongoing
		// opened batch and leave db state correctly.
		require.Equal(t, 0, tableRowCount(t, pool, "foo"))
	})
}

func TestRegisterTable(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		tableUUID := uuid.New()
		createStmt := "create table bar (zar text)"
		err = b.InsertTable(ctx, tableUUID, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", "type", createStmt)
		require.NoError(t, err)

		require.NoError(t, b.Commit(ctx))
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		// Check that the table was registered in the system-table.
		systemStore, err := system.New(pool)
		require.NoError(t, err)
		table, err := systemStore.GetTable(ctx, tableUUID)
		require.NoError(t, err)
		require.Equal(t, tableUUID, table.UUID)
		require.Equal(t, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", table.Controller)
		require.Equal(t, "type", table.Type)
		require.NotEqual(t, new(time.Time), table.CreatedAt) // CreatedAt is not the zero value

		// Check that the user table was created.
		ok := existsTableWithName(t, pool, "bar")
		require.True(t, ok)
	})
	t.Run("wrong create stmt", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		tableUUID := uuid.New()
		createStmt := "create tablez bar (zar text)"
		err = b.InsertTable(ctx, tableUUID, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", "type", createStmt)
		require.Error(t, err)

		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		systemTableRowCount := tableRowCount(t, pool, "system_tables")
		require.Equal(t, 0, systemTableRowCount)

		ok := existsTableWithName(t, pool, "bar")
		require.False(t, ok)
	})
}

func tableRowCount(t *testing.T, pool *pgxpool.Pool, tableName string) int {
	t.Helper()

	q := "select count(*) from " + tableName
	row := pool.QueryRow(context.Background(), q)
	var rowCount int
	err := row.Scan(&rowCount)
	if err == pgx.ErrNoRows {
		return 0
	}
	require.NoError(t, err)

	return rowCount
}

func existsTableWithName(t *testing.T, pool *pgxpool.Pool, tableName string) bool {
	t.Helper()
	q := `SELECT 1 FROM information_schema.tables  WHERE table_name = $1`
	row := pool.QueryRow(context.Background(), q, tableName)
	var dummy int
	err := row.Scan(&dummy)
	if err == pgx.ErrNoRows {
		return false
	}
	require.NoError(t, err)
	return true
}

func newTxnProcessorWithTable(t *testing.T) (*TblTxnProcessor, *pgxpool.Pool) {
	t.Helper()

	url, err := tests.PostgresURL()
	require.NoError(t, err)

	txnp, err := NewTxnProcessor(url)
	require.NoError(t, err)

	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	require.NoError(t, err)

	// Boostrap system store to run the db migrations.
	_, err = system.New(pool)
	require.NoError(t, err)

	createTableQuery := "create table foo (name text)"
	_, err = pool.Exec(ctx, createTableQuery)
	require.NoError(t, err)

	return txnp, pool
}
