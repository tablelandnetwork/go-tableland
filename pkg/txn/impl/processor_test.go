package impl

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/txn"
	"github.com/textileio/go-tableland/tests"
)

func TestRunSQL(t *testing.T) {
	t.Parallel()
	t.Run("single-query", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		wq1 := mustWriteStmt(t, `insert into foo_100 values ('one')`)
		err = b.ExecWriteQueries(ctx, []parsing.SugaredWriteStmt{wq1})
		require.NoError(t, err)

		require.NoError(t, b.Commit(ctx))
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		require.Equal(t, 1, tableRowCountT100(t, pool))
	})

	t.Run("multiple queries", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		{
			wq1 := mustWriteStmt(t, `insert into foo_100 values ('wq1one')`)
			err = b.ExecWriteQueries(ctx, []parsing.SugaredWriteStmt{wq1})
			require.NoError(t, err)
		}
		{
			wq1 := mustWriteStmt(t, `insert into foo_100 values ('wq1two')`)
			wq2 := mustWriteStmt(t, `insert into foo_100 values ('wq2three')`)
			err = b.ExecWriteQueries(ctx, []parsing.SugaredWriteStmt{wq1, wq2})
			require.NoError(t, err)
		}
		{
			wq1 := mustWriteStmt(t, `insert into foo_100 values ('wq1four')`)
			err = b.ExecWriteQueries(ctx, []parsing.SugaredWriteStmt{wq1})
			require.NoError(t, err)
		}

		require.NoError(t, b.Commit(ctx))
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		require.Equal(t, 4, tableRowCountT100(t, pool))
	})

	t.Run("multiple with single failure", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		{
			wq1_1 := mustWriteStmt(t, `insert into foo_100 values ('onez')`)
			err = b.ExecWriteQueries(ctx, []parsing.SugaredWriteStmt{wq1_1})
			require.NoError(t, err)
		}
		{
			wq2_1 := mustWriteStmt(t, `insert into foo_100 values ('twoz')`)
			wq2_2 := mustWriteStmt(t, `insert into foo_101 values ('threez')`)
			err = b.ExecWriteQueries(ctx, []parsing.SugaredWriteStmt{wq2_1, wq2_2})
			require.Error(t, err)
		}
		{
			wq3_1 := mustWriteStmt(t, `insert into foo_100 values ('fourz')`)
			err = b.ExecWriteQueries(ctx, []parsing.SugaredWriteStmt{wq3_1})
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
		require.Equal(t, 2, tableRowCountT100(t, pool))
	})

	t.Run("with abrupt close", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		{
			wq1_1 := mustWriteStmt(t, `insert into foo_100 values ('one')`)
			err = b.ExecWriteQueries(ctx, []parsing.SugaredWriteStmt{wq1_1})
			require.NoError(t, err)
		}
		{
			wq2_1 := mustWriteStmt(t, `insert into foo_100 values ('two')`)
			wq2_2 := mustWriteStmt(t, `insert into foo_100 values ('three')`)
			err = b.ExecWriteQueries(ctx, []parsing.SugaredWriteStmt{wq2_1, wq2_2})
			require.NoError(t, err)
		}

		// Note: we don't do a Commit() call, thus all should be rollbacked.
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		// The opened batch wasn't txnp.CloseBatch(), but we simply
		// closed the whole store. This should rollback any ongoing
		// opened batch and leave db state correctly.
		require.Equal(t, 0, tableRowCountT100(t, pool))
	})
}

func TestRegisterTable(t *testing.T) {
	parser := parserimpl.New([]string{}, 0, 0)
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessor(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		id, err := tableland.NewTableID("100")
		require.NoError(t, err)
		createStmt, err := parser.ValidateCreateTable("create table bar (zar text)")
		require.NoError(t, err)
		err = b.InsertTable(ctx, id, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", "descrip", createStmt)
		require.NoError(t, err)

		require.NoError(t, b.Commit(ctx))
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		// Check that the table was registered in the system-table.
		systemStore, err := system.New(pool)
		require.NoError(t, err)
		table, err := systemStore.GetTable(ctx, id)
		require.NoError(t, err)
		require.Equal(t, id, table.ID)
		require.Equal(t, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", table.Controller)
		require.Equal(t, "descrip", table.Description)
		// sha256(zar:text) = 926b64e777db62e4d9e9007dc51e3974fce37c50f456177bec98cd797bc819f8
		require.Equal(t, "926b64e777db62e4d9e9007dc51e3974fce37c50f456177bec98cd797bc819f8", table.Structure)
		require.Equal(t, "bar", table.Name)
		require.NotEqual(t, new(time.Time), table.CreatedAt) // CreatedAt is not the zero value

		// Check that the user table was created.
		ok := existsTableWithName(t, pool, "_100")
		require.True(t, ok)
	})
}

func TestTableRowCountLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	rowLimit := 10
	txnp, pool := newTxnProcessorWithTable(t, rowLimit)

	// Helper func to insert a row and return an error if happened.
	insertRow := func(t *testing.T) error {
		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		q := mustWriteStmt(t, `insert into foo_100 values ('one')`)
		err = b.ExecWriteQueries(ctx, []parsing.SugaredWriteStmt{q})
		if err == nil {
			require.NoError(t, b.Commit(ctx))
		}
		require.NoError(t, b.Close(ctx))
		return err
	}

	// Insert up to 10 rows should succeed.
	for i := 0; i < rowLimit; i++ {
		require.NoError(t, insertRow(t))
	}
	require.Equal(t, rowLimit, tableRowCountT100(t, pool))

	// The next insert should fail.
	var errRowCount *txn.ErrRowCountExceeded
	require.ErrorAs(t, insertRow(t), &errRowCount)
	require.Equal(t, rowLimit, errRowCount.BeforeRowCount)
	require.Equal(t, rowLimit+1, errRowCount.AfterRowCount)

	require.NoError(t, txnp.Close(ctx))
}

func tableRowCountT100(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()

	q := "select count(*) from _100"
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

func newTxnProcessor(t *testing.T, rowsLimit int) (*TblTxnProcessor, *pgxpool.Pool) {
	t.Helper()

	url, err := tests.PostgresURL()
	require.NoError(t, err)

	txnp, err := NewTxnProcessor(url, rowsLimit)
	require.NoError(t, err)

	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	require.NoError(t, err)

	// Boostrap system store to run the db migrations.
	_, err = system.New(pool)
	require.NoError(t, err)
	return txnp, pool
}

func newTxnProcessorWithTable(t *testing.T, rowsLimit int) (*TblTxnProcessor, *pgxpool.Pool) {
	t.Helper()

	txnp, pool := newTxnProcessor(t, rowsLimit)
	ctx := context.Background()

	b, err := txnp.OpenBatch(ctx)
	require.NoError(t, err)
	id, err := tableland.NewTableID("100")
	require.NoError(t, err)
	parser := parserimpl.New([]string{}, 0, 0)
	createStmt, err := parser.ValidateCreateTable("create table foo (zar text)")
	require.NoError(t, err)
	err = b.InsertTable(ctx, id, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", "descrip", createStmt)
	require.NoError(t, err)

	require.NoError(t, b.Commit(ctx))
	require.NoError(t, b.Close(ctx))

	return txnp, pool
}

func mustWriteStmt(t *testing.T, q string) parsing.SugaredWriteStmt {
	t.Helper()
	p := parserimpl.New([]string{"system_", "registry"}, 0, 0)
	_, wss, err := p.ValidateRunSQL(q)
	require.NoError(t, err)
	require.Len(t, wss, 1)
	return wss[0]
}
