package impl

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/tests"
)

// Random address for testing. The value isn't important
// because the ACL is mocked.
var (
	controller = common.HexToAddress("0x07dfFc57AA386D2b239CaBE8993358DF20BAFBE2")
	chainID    = 1337
)

func TestReceiptExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	txnp, _, _ := newExecutorWithTable(t, 0)

	txnHash := "0x0000000000000000000000000000000000000000000000000000000000001234"

	b, err := txnp.OpenBatch(ctx)
	require.NoError(t, err)
	ok, err := b.TxnReceiptExists(ctx, common.HexToHash(txnHash))
	require.NoError(t, err)
	require.False(t, ok)
	require.NoError(t, b.Commit())
	require.NoError(t, b.Close())

	b, err = txnp.OpenBatch(ctx)
	require.NoError(t, err)
	err = b.SaveTxnReceipts(ctx, []eventprocessor.Receipt{
		{
			ChainID:     tableland.ChainID(chainID),
			BlockNumber: 100,
			TxnHash:     txnHash,
		},
	})
	require.NoError(t, err)
	require.NoError(t, b.Commit())
	require.NoError(t, b.Close())

	b, err = txnp.OpenBatch(ctx)
	require.NoError(t, err)
	ok, err = b.TxnReceiptExists(ctx, common.HexToHash(txnHash))
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, b.Commit())
	require.NoError(t, b.Close())

	require.NoError(t, txnp.Close(ctx))
}

func TestExecWriteQueriesWithPolicies(t *testing.T) {
	t.Parallel()

	t.Run("insert-not-allowed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _, _ := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		policy := policyFactory(policyData{
			isInsertAllowed: false,
		})

		wq := mustWriteStmt(t, `insert into foo_1337_100 values ('one');`)

		// set the controller to anything other than zero
		err = b.SetController(ctx, wq.GetTableID(), common.HexToAddress("0x1"))
		require.NoError(t, err)

		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq}, true, policy)
		var errQueryExecution *executor.ErrQueryExecution
		require.ErrorAs(t, err, &errQueryExecution)
		require.ErrorContains(t, err, "insert is not allowed by policy")
	})

	t.Run("update-not-allowed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _, _ := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		policy := policyFactory(policyData{
			isUpdateAllowed: false,
		})

		wq := mustWriteStmt(t, `update foo_1337_100 set zar = 'three';`)

		// set the controller to anything other than zero
		err = b.SetController(ctx, wq.GetTableID(), common.HexToAddress("0x1"))
		require.NoError(t, err)

		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq}, true, policy)
		var errQueryExecution *executor.ErrQueryExecution
		require.ErrorAs(t, err, &errQueryExecution)
		require.ErrorContains(t, err, "update is not allowed by policy")
	})

	t.Run("delete-not-allowed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _, _ := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		policy := policyFactory(policyData{
			isDeleteAllowed: false,
		})

		wq := mustWriteStmt(t, `DELETE FROM foo_1337_100`)

		// set the controller to anything other than zero
		err = b.SetController(ctx, wq.GetTableID(), common.HexToAddress("0x1"))
		require.NoError(t, err)

		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq}, true, policy)
		var errQueryExecution *executor.ErrQueryExecution
		require.ErrorAs(t, err, &errQueryExecution)
		require.ErrorContains(t, err, "delete is not allowed by policy")
	})

	t.Run("update-column-not-allowed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _, _ := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		policy := policyFactory(policyData{
			isUpdateAllowed:  true,
			updatableColumns: []string{"zaz"}, // zaz instead of zar
		})

		// tries to update zar and not zaz
		wq := mustWriteStmt(t, `update foo_1337_100 set zar = 'three';`)

		// set the controller to anything other than zero
		err = b.SetController(ctx, wq.GetTableID(), common.HexToAddress("0x1"))
		require.NoError(t, err)

		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq}, true, policy)
		var errQueryExecution *executor.ErrQueryExecution
		require.ErrorAs(t, err, &errQueryExecution)
		require.ErrorContains(t, err, "column zar is not allowed")
	})

	t.Run("update-where-policy", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _, pool := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		// start with two rows
		wq1 := mustWriteStmt(t, `insert into foo_1337_100 values ('one');`)
		wq2 := mustWriteStmt(t, `insert into foo_1337_100 values ('two');`)

		// set the controller to anything other than zero
		err = b.SetController(ctx, wq2.GetTableID(), common.HexToAddress("0x1"))
		require.NoError(t, err)

		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq1, wq2}, true, tableland.AllowAllPolicy{})
		require.NoError(t, err)

		policy := policyFactory(policyData{
			isUpdateAllowed:  true,
			whereClause:      "zar = 'two'",
			updatableColumns: []string{"zar"},
		})

		// send an update that updates all rows with a policy to restricts the update
		wq3 := mustWriteStmt(t, `update foo_1337_100 set zar = 'three'`)
		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq3}, true, policy)
		require.NoError(t, err)

		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, txnp.Close(ctx))

		// there should be only one row updated
		require.Equal(t, 1, tableRowCountT100(t, pool, "select count(*) from foo_1337_100 WHERE zar = 'three'"))
	})
}

func TestRegisterTable(t *testing.T) {
	t.Parallel()

	parser := newParser(t, []string{})
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, dbURL := newExecutor(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		id, err := tableland.NewTableID("100")
		require.NoError(t, err)
		createStmt, err := parser.ValidateCreateTable("create table bar_1337 (zar text)", 1337)
		require.NoError(t, err)
		err = b.InsertTable(ctx, id, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", createStmt)
		require.NoError(t, err)

		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, txnp.Close(ctx))

		// Check that the table was registered in the system-table.
		systemStore, err := system.New(dbURL, tableland.ChainID(chainID))
		require.NoError(t, err)
		table, err := systemStore.GetTable(ctx, id)
		require.NoError(t, err)
		require.Equal(t, id, table.ID)
		require.Equal(t, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", table.Controller)
		// echo -n zar:TEXT | shasum -a 256
		require.Equal(t, "7ec5320c16e06e90af5e7131ff0c80d4b0a08fcd62aa6e38ad8d6843bc480d09", table.Structure)
		require.Equal(t, "bar", table.Prefix)
		require.NotEqual(t, new(time.Time), table.CreatedAt) // CreatedAt is not the zero value

		// Check that the user table was created.
		ok := existsTableWithName(t, dbURL, "bar_1337_100")
		require.True(t, ok)
	})
}

func TestTableRowCountLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	rowLimit := 10
	txnp, _, pool := newExecutorWithTable(t, rowLimit)

	// Helper func to insert a row and return an error if happened.
	insertRow := func(t *testing.T) error {
		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		q := mustWriteStmt(t, `insert into foo_1337_100 values ('one')`)
		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{q}, true, tableland.AllowAllPolicy{})
		if err == nil {
			require.NoError(t, b.Commit())
		}
		require.NoError(t, b.Close())
		return err
	}

	// Insert up to 10 rows should succeed.
	for i := 0; i < rowLimit; i++ {
		require.NoError(t, insertRow(t))
	}
	require.Equal(t, rowLimit, tableRowCountT100(t, pool, "select count(*) from foo_1337_100"))

	// The next insert should fail.
	var errQueryExecution *executor.ErrQueryExecution
	err := insertRow(t)
	require.ErrorAs(t, err, &errQueryExecution)
	require.ErrorContains(t, err,
		fmt.Sprintf("table maximum row count exceeded (before %d, after %d)", rowLimit, rowLimit+1),
	)

	require.NoError(t, txnp.Close(ctx))
}

func TestSetController(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tableID := tableland.TableID(*big.NewInt(100))

	t.Run("controller-is-not-set-default", func(t *testing.T) {
		t.Parallel()
		_, _, db := newExecutorWithTable(t, 0)

		// Let's test first that the controller is not set (it's the default behavior)
		tx, err := db.BeginTx(ctx, &sql.TxOptions{
			Isolation: sql.LevelSerializable,
			ReadOnly:  false,
		})
		require.NoError(t, err)
		controller, err := getController(ctx, tx, tableland.ChainID(chainID), tableID)
		require.NoError(t, err)
		require.Equal(t, "", controller)
		require.NoError(t, tx.Commit())
	})

	t.Run("foreign-key-constraint", func(t *testing.T) {
		t.Parallel()
		txnp, _, _ := newExecutorWithTable(t, 0)

		// table id different than 100 violates foreign key
		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)
		err = b.SetController(ctx, tableland.TableID(*big.NewInt(1)), common.HexToAddress("0x01"))
		require.NoError(t, b.Commit())
		require.Error(t, err)
		var errQueryExecution *executor.ErrQueryExecution
		require.NotErrorIs(t, err, errQueryExecution)
		require.Contains(t, err.Error(), "FOREIGN KEY constraint failed")

		require.NoError(t, b.Close())
		require.NoError(t, txnp.Close(ctx))
	})

	t.Run("set-unset-controller", func(t *testing.T) {
		t.Parallel()
		txnp, _, db := newExecutorWithTable(t, 0)

		// sets
		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)
		err = b.SetController(ctx, tableID, common.HexToAddress("0x01"))
		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, err)

		tx, err := db.BeginTx(ctx, &sql.TxOptions{
			Isolation: sql.LevelSerializable,
			ReadOnly:  false,
		})
		require.NoError(t, err)
		controller, err := getController(ctx, tx, tableland.ChainID(chainID), tableID)
		require.NoError(t, err)
		require.Equal(t, "0x0000000000000000000000000000000000000001", controller)
		require.NoError(t, tx.Commit())

		// unsets
		b, err = txnp.OpenBatch(ctx)
		require.NoError(t, err)
		err = b.SetController(ctx, tableID, common.HexToAddress("0x0"))
		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, err)

		tx, err = db.BeginTx(ctx, &sql.TxOptions{
			Isolation: sql.LevelSerializable,
			ReadOnly:  false,
		})
		require.NoError(t, err)
		controller, err = getController(ctx, tx, tableland.ChainID(chainID), tableID)
		require.NoError(t, err)
		require.Equal(t, "", controller)
		require.NoError(t, tx.Commit())

		require.NoError(t, txnp.Close(ctx))
	})

	t.Run("upsert", func(t *testing.T) {
		t.Parallel()
		txnp, _, db := newExecutorWithTable(t, 0)

		{
			b, err := txnp.OpenBatch(ctx)
			require.NoError(t, err)
			err = b.SetController(ctx, tableID, common.HexToAddress("0x01"))
			require.NoError(t, b.Commit())
			require.NoError(t, err)
			require.NoError(t, b.Close())
		}

		{
			b, err := txnp.OpenBatch(ctx)
			require.NoError(t, err)
			err = b.SetController(ctx, tableID, common.HexToAddress("0x02"))
			require.NoError(t, b.Commit())
			require.NoError(t, err)
			require.NoError(t, b.Close())
		}

		tx, err := db.BeginTx(ctx, &sql.TxOptions{
			Isolation: sql.LevelSerializable,
			ReadOnly:  false,
		})
		require.NoError(t, err)
		controller, err := getController(ctx, tx, tableland.ChainID(chainID), tableID)
		require.NoError(t, err)
		require.Equal(t, "0x0000000000000000000000000000000000000002", controller)
		require.NoError(t, tx.Commit())

		require.NoError(t, txnp.Close(ctx))
	})
}

func TestWithCheck(t *testing.T) {
	t.Parallel()
	t.Run("insert-with-check-not-satistifed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _, pool := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		policy := policyFactory(policyData{
			isInsertAllowed: true,
			withCheck:       "zar = 'two'",
		})

		wq := mustWriteStmt(t, `insert into foo_1337_100 values ('one')`)

		// set the controller to anything other than zero
		err = b.SetController(ctx, wq.GetTableID(), common.HexToAddress("0x1"))
		require.NoError(t, err)

		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq}, true, policy)
		var errQueryExecution *executor.ErrQueryExecution
		require.ErrorAs(t, err, &errQueryExecution)
		require.ErrorContains(t, err, "number of affected rows 1 does not match auditing count 0")

		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, txnp.Close(ctx))

		require.Equal(t, 0, tableRowCountT100(t, pool, "select count(*) from foo_1337_100"))
	})

	t.Run("update-with-check-not-satistifed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _, pool := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		wq1 := mustWriteStmt(t, `insert into foo_1337_100 values ('one')`)

		// set the controller to anything other than zero
		err = b.SetController(ctx, wq1.GetTableID(), common.HexToAddress("0x1"))
		require.NoError(t, err)

		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq1}, true, &tableland.AllowAllPolicy{})
		require.Nil(t, err)

		wq2 := mustWriteStmt(t, `update foo_1337_100 SET zar = 'three'`)
		policy := policyFactory(policyData{
			isUpdateAllowed: true,
			withCheck:       "zar = 'two'",
		})

		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq2}, true, policy)
		var errQueryExecution *executor.ErrQueryExecution
		require.ErrorAs(t, err, &errQueryExecution)
		require.ErrorContains(t, err, "number of affected rows 1 does not match auditing count 0")

		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, txnp.Close(ctx))

		require.Equal(t, 1, tableRowCountT100(t, pool, "select count(*) from foo_1337_100 WHERE zar = 'one'"))
		require.Equal(t, 0, tableRowCountT100(t, pool, "select count(*) from foo_1337_100 WHERE zar = 'three'"))
	})

	t.Run("insert-with-check-satistifed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _, pool := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		policy := policyFactory(policyData{
			isInsertAllowed: true,
			withCheck:       "zar in ('one', 'two')",
		})

		wq1 := mustWriteStmt(t, `insert into foo_1337_100 values ('one')`)
		wq2 := mustWriteStmt(t, `insert into foo_1337_100 values ('two')`)

		// set the controller to anything other than zero
		err = b.SetController(ctx, wq1.GetTableID(), common.HexToAddress("0x1"))
		require.NoError(t, err)

		_ = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq1, wq2}, true, policy)
		require.Nil(t, err)

		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, txnp.Close(ctx))

		require.Equal(t, 2, tableRowCountT100(t, pool, "select count(*) from foo_1337_100"))
	})

	t.Run("row-count-limit-withcheck", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		rowLimit := 10
		txnp, _, pool := newExecutorWithTable(t, rowLimit)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		// set the controller to anything other than zero
		err = b.SetController(ctx, tableland.TableID(*big.NewInt(100)), common.HexToAddress("0x1"))
		require.NoError(t, err)

		require.NoError(t, b.Close())

		// Helper func to insert a row and return an error if happened.
		insertRow := func(t *testing.T) error {
			b, err := txnp.OpenBatch(ctx)
			require.NoError(t, err)

			policy := policyFactory(policyData{
				isInsertAllowed: true,
				withCheck:       "zar in ('one')",
			})

			q := mustWriteStmt(t, `insert into foo_1337_100 values ('one')`)

			err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{q}, true, policy)
			if err == nil {
				require.NoError(t, b.Commit())
			}
			require.NoError(t, b.Close())
			return err
		}

		// Insert up to 10 rows should succeed.
		for i := 0; i < rowLimit; i++ {
			require.NoError(t, insertRow(t))
		}
		require.Equal(t, rowLimit, tableRowCountT100(t, pool, "select count(*) from foo_1337_100"))

		// The next insert should fail.
		var errQueryExecution *executor.ErrQueryExecution
		err = insertRow(t)
		require.ErrorAs(t, err, &errQueryExecution)
		require.ErrorContains(t, err,
			fmt.Sprintf("table maximum row count exceeded (before %d, after %d)", rowLimit, rowLimit+1),
		)

		require.NoError(t, txnp.Close(ctx))
	})
}

func TestChangeTableOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tableID := tableland.TableID(*big.NewInt(100))
	txnp, _, db := newExecutorWithTable(t, 0)

	require.Equal(t, 1,
		tableRowCountT100(
			t,
			db,
			fmt.Sprintf(
				"select count(1) from registry WHERE controller = '%s' and id = %s and chain_id = %d",
				"0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF",
				tableID.String(),
				chainID,
			),
		))

	b, err := txnp.OpenBatch(ctx)
	require.NoError(t, err)

	// change table's owner
	err = b.ChangeTableOwner(ctx, tableID, controller)
	require.NoError(t, err)
	require.NoError(t, b.Commit())
	require.NoError(t, b.Close())
	require.NoError(t, txnp.Close(ctx))

	require.Equal(t, 1,
		tableRowCountT100(
			t,
			db,
			fmt.Sprintf(
				"select count(1) from registry WHERE controller = '%s' and id = %s and chain_id = %d",
				controller.Hex(),
				tableID.String(),
				chainID,
			),
		))
}

func tableRowCountT100(t *testing.T, pool *sql.DB, query string) int {
	t.Helper()

	row := pool.QueryRowContext(context.Background(), query)
	var rowCount int
	err := row.Scan(&rowCount)
	if err == sql.ErrNoRows {
		return 0
	}
	require.NoError(t, err)

	return rowCount
}

func existsTableWithName(t *testing.T, dbURL string, tableName string) bool {
	t.Helper()

	pool, err := sql.Open("sqlite3", dbURL)
	require.NoError(t, err)
	q := `SELECT 1 FROM sqlite_master  WHERE type='table' AND name = ?1`
	row := pool.QueryRow(q, tableName)
	var dummy int
	err = row.Scan(&dummy)
	if err == sql.ErrNoRows {
		return false
	}
	require.NoError(t, err)
	return true
}

func newExecutor(t *testing.T, rowsLimit int) (*Executor, string) {
	t.Helper()

	dbURI := tests.Sqlite3URI()

	parser := newParser(t, []string{})
	exec, err := NewExecutor(1337, dbURI, parser, rowsLimit, &aclMock{})
	require.NoError(t, err)

	// Boostrap system store to run the db migrations.
	_, err = system.New(dbURI, tableland.ChainID(chainID))
	require.NoError(t, err)
	return exec, dbURI
}

func newExecutorWithTable(t *testing.T, rowsLimit int) (*Executor, string, *sql.DB) {
	t.Helper()

	ex, dbURL := newExecutor(t, rowsLimit)
	ctx := context.Background()

	ibs, err := ex.NewBlockScope(ctx, 0, "0xFAKETXNHASH")
	require.NoError(t, err)
	bs := ibs.(*blockScope)

	// Pre-bake a table with ID 100.
	id, err := tableland.NewTableID("100")
	require.NoError(t, err)
	require.NoError(t, err)
	res, err := bs.ExecuteTxnEvents(ctx, eventfeed.TxnEvents{
		TxnHash: common.HexToHash("0xF1"),
		Events: []interface{}{
			&ethereum.ContractCreateTable{
				Owner:     controller,
				TableId:   id.ToBigInt(),
				Statement: "create table foo_1337 (zar text)",
			},
		},
	})
	require.NoError(t, err)
	require.Nil(t, res.Error)
	require.NotNil(t, res.TableID)

	require.NoError(t, bs.Commit())
	require.NoError(t, bs.Close())

	pool, err := sql.Open("sqlite3", dbURL)
	require.NoError(t, err)

	return ex, dbURL, pool
}

// TODO(jsign): evaluate later if these methods "must*" can be deleted.
func mustWriteStmt(t *testing.T, q string) parsing.MutatingStmt {
	t.Helper()
	p := newParser(t, []string{"system_", "registry"})
	wss, err := p.ValidateMutatingQuery(q, 1337)
	require.NoError(t, err)
	require.Len(t, wss, 1)
	return wss[0]
}

func mustGrantStmt(t *testing.T, q string) parsing.MutatingStmt {
	t.Helper()
	p := newParser(t, []string{"system_", "registry"})
	wss, err := p.ValidateMutatingQuery(q, 1337)
	require.NoError(t, err)
	require.Len(t, wss, 1)
	return wss[0]
}

func newParser(t *testing.T, prefixes []string) parsing.SQLValidator {
	t.Helper()
	p, err := parserimpl.New(prefixes)
	require.NoError(t, err)
	return p
}

type aclMock struct{}

func (acl *aclMock) CheckPrivileges(
	ctx context.Context,
	tx *sql.Tx,
	controller common.Address,
	id tableland.TableID,
	op tableland.Operation,
) (bool, error) {
	return true, nil
}

type policyData struct {
	isInsertAllowed  bool
	isUpdateAllowed  bool
	isDeleteAllowed  bool
	whereClause      string
	updatableColumns []string
	withCheck        string
}

func policyFactory(data policyData) tableland.Policy {
	return mockPolicy{data}
}

type mockPolicy struct {
	policyData
}

func (p mockPolicy) IsInsertAllowed() bool      { return p.policyData.isInsertAllowed }
func (p mockPolicy) IsUpdateAllowed() bool      { return p.policyData.isUpdateAllowed }
func (p mockPolicy) IsDeleteAllowed() bool      { return p.policyData.isDeleteAllowed }
func (p mockPolicy) WhereClause() string        { return p.policyData.whereClause }
func (p mockPolicy) UpdatableColumns() []string { return p.policyData.updatableColumns }
func (p mockPolicy) WithCheck() string          { return p.policyData.withCheck }
