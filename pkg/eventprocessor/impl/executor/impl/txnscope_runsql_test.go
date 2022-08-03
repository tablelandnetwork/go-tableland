package impl

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
)

func TestRunSQL_OneEventPerTxn(t *testing.T) {
	t.Parallel()
	t.Run("one insert", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		ex, dbURI := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		assertExecTxnWithRunSQLEvents(t, bs, 100, []string{`insert into foo_1337_100 values ('one')`})

		require.NoError(t, bs.Commit())
		require.NoError(t, bs.Close())
		require.NoError(t, ex.Close(ctx))

		require.Equal(t, 1, tableRowCountT100(t, dbURI, "select count(*) from foo_1337_100"))
	})

	t.Run("multiple inserts", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, dbURI := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		assertExecTxnWithRunSQLEvents(t, bs, 100, []string{`insert into foo_1337_100 values ('wq1one')`})
		assertExecTxnWithRunSQLEvents(t, bs, 100, []string{`insert into foo_1337_100 values ('wq2one');insert into foo_1337_100 values ('wq2two')`}) //nolint
		assertExecTxnWithRunSQLEvents(t, bs, 100, []string{`insert into foo_1337_100 values ('wq3one')`})

		require.NoError(t, bs.Commit())
		require.NoError(t, bs.Close())
		require.NoError(t, ex.Close(ctx))

		// 3 txns each with one event with a total of 4 inserts.
		require.Equal(t, 4, tableRowCountT100(t, dbURI, "select count(*) from foo_1337_100"))
	})

	t.Run("multiple with single failure", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, dbURI := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		assertExecTxnWithRunSQLEvents(t, bs, 100, []string{`insert into foo_1337_100 values ('onez')`})
		res, err := execTxnWithRunSQLEvents(t, bs, 100, []string{`insert into foo_1337_100 values ('twoz');insert into foo_1337_101 values ('threez')`})
		require.NoError(t, err)
		require.NotNil(t, res.Error)
		require.Equal(t, 0, *res.ErrorEventIdx)
		assertExecTxnWithRunSQLEvents(t, bs, 100, []string{`insert into foo_1337_100 values ('fourz')`})

		require.NoError(t, bs.Commit())
		require.NoError(t, bs.Close())
		require.NoError(t, ex.Close(ctx))

		// We executed 3 transactions each with one RunSQL event.
		// The first and third transaction succeeded. The second failed since one of its queries reference a
		// wrong table.
		//
		// We check that we see 2 inserted rows, from the first and third transaction.
		// Despite the first query of the second transaction was correct, it must be rollbacked since the second
		// query wasn't.
		require.Equal(t, 2, tableRowCountT100(t, dbURI, "select count(*) from foo_1337_100"))
	})

	t.Run("with abrupt close", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, dbURI := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		assertExecTxnWithRunSQLEvents(t, bs, 100, []string{`insert into foo_1337_100 values ('one')`})
		assertExecTxnWithRunSQLEvents(t, bs, 100, []string{`insert into foo_1337_100 values ('two');insert into foo_1337_100 values ('three')`})

		// We **don't** Commit(), thus all should be rollbacked.
		require.NoError(t, bs.Close())
		require.NoError(t, ex.Close(ctx))

		// The opened batch wasn't txnp.CloseBatch(), but we simply
		// closed the whole store. This should rollback any ongoing
		// opened batch and leave db state correctly.
		require.Equal(t, 0, tableRowCountT100(t, dbURI, "select count(*) from foo_1337_100"))
	})

	t.Run("one grant", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, dbURI := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		q := "grant insert, update, delete on foo_1337_100 to '0xd43c59d5694ec111eb9e986c233200b14249558d', '0x4afe8e30db4549384b0a05bb796468b130c7d6e0'" //nolint
		assertExecTxnWithRunSQLEvents(t, bs, 100, []string{q})

		require.NoError(t, bs.Commit())
		require.NoError(t, bs.Close())
		require.NoError(t, ex.Close(ctx))

		require.NoError(t, err)
		ss := mustGrantStmt(t, q).(parsing.GrantStmt)
		for _, role := range ss.GetRoles() {
			// Check that an entry was inserted in the system_acl table for each row.
			systemStore, err := system.New(dbURI, tableland.ChainID(chainID))
			require.NoError(t, err)
			aclRow, err := systemStore.GetACLOnTableByController(ctx, ss.GetTableID(), role.String())
			require.NoError(t, err)
			require.Equal(t, ss.GetTableID(), aclRow.TableID)
			require.Equal(t, role.String(), aclRow.Controller)
			require.ElementsMatch(t, ss.GetPrivileges(), aclRow.Privileges)
		}
	})

	t.Run("grant upsert", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, dbURI := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		q := "grant insert on foo_1337_100 to '0xd43c59d5694ec111eb9e986c233200b14249558d', '0x4afe8e30db4549384b0a05bb796468b130c7d6e0';" //nolint
		// add the update privilege for role 0xd43c59d5694ec111eb9e986c233200b14249558d
		q += "grant update on foo_1337_100 to '0xd43c59d5694ec111eb9e986c233200b14249558d';"
		// add the delete privilege (and mistakenly the insert) grant for role 0x4afe8e30db4549384b0a05bb796468b130c7d6e0
		q += "grant insert, delete on foo_1337_100 to '0x4afe8e30db4549384b0a05bb796468b130c7d6e0'"
		assertExecTxnWithRunSQLEvents(t, bs, 100, []string{q})

		require.NoError(t, bs.Commit())
		require.NoError(t, bs.Close())
		require.NoError(t, ex.Close(ctx))

		systemStore, err := system.New(dbURI, tableland.ChainID(chainID))
		require.NoError(t, err)

		tableID, _ := tableland.NewTableID("100")
		{
			aclRow, err := systemStore.GetACLOnTableByController(
				ctx,
				tableID,
				"0xD43C59d5694eC111Eb9e986C233200b14249558D")
			require.NoError(t, err)
			require.Equal(t, tableID, aclRow.TableID)
			require.Equal(t, "0xD43C59d5694eC111Eb9e986C233200b14249558D", aclRow.Controller)
			require.ElementsMatch(t, tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate}, aclRow.Privileges)
		}

		{
			aclRow, err := systemStore.GetACLOnTableByController(
				ctx,
				tableID,
				"0x4afE8e30DB4549384b0a05bb796468B130c7D6E0")
			require.NoError(t, err)
			require.Equal(t, tableID, aclRow.TableID)
			require.Equal(t, "0x4afE8e30DB4549384b0a05bb796468B130c7D6E0", aclRow.Controller)
			require.ElementsMatch(t, tableland.Privileges{tableland.PrivInsert, tableland.PrivDelete}, aclRow.Privileges)
		}
	})

	t.Run("grant revoke", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, dbURI := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		q := "grant insert, update, delete on foo_1337_100 to '0xd43c59d5694ec111eb9e986c233200b14249558d';"
		q += "revoke insert, delete on foo_1337_100 from '0xd43c59d5694ec111eb9e986c233200b14249558d';"
		assertExecTxnWithRunSQLEvents(t, bs, 100, []string{q})

		require.NoError(t, bs.Commit())
		require.NoError(t, bs.Close())
		require.NoError(t, ex.Close(ctx))

		systemStore, err := system.New(dbURI, tableland.ChainID(chainID))
		require.NoError(t, err)

		tableID, _ := tableland.NewTableID("100")
		{
			aclRow, err := systemStore.GetACLOnTableByController(
				ctx,
				tableID,
				"0xD43C59d5694eC111Eb9e986C233200b14249558D")
			require.NoError(t, err)
			require.Equal(t, tableID, aclRow.TableID)
			require.Equal(t, "0xD43C59d5694eC111Eb9e986C233200b14249558D", aclRow.Controller)
			require.ElementsMatch(t, tableland.Privileges{tableland.PrivUpdate}, aclRow.Privileges)
		}
	})
}

func TestRunSQL_WriteQueriesWithPolicies(t *testing.T) {
	t.Parallel()

	t.Run("insert not allowed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, _ := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		// set the controller to anything other than zero
		assertExecTxnWithSetController(t, bs, 100, "0x1")

		policy := ethereum.ITablelandControllerPolicy{AllowInsert: false}
		res, err := execTxnWithRunSQLEventsAndPolicy(
			t, bs, 100, []string{`insert into foo_1337_100 values ('one');`}, policy)
		require.NoError(t, err)
		require.Contains(t, *res.Error, "insert is not allowed by policy")
	})

	t.Run("update not allowed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, _ := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		// set the controller to anything other than zero
		assertExecTxnWithSetController(t, bs, 100, "0x1")
		require.NoError(t, err)

		policy := ethereum.ITablelandControllerPolicy{AllowUpdate: false}
		res, err := execTxnWithRunSQLEventsAndPolicy(
			t, bs, 100, []string{`update foo_1337_100 set zar = 'three';`}, policy)
		require.NoError(t, err)
		require.Contains(t, *res.Error, "update is not allowed by policy")
	})

	t.Run("delete not allowed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, _ := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		// set the controller to anything other than zero
		assertExecTxnWithSetController(t, bs, 100, "0x1")
		require.NoError(t, err)

		policy := ethereum.ITablelandControllerPolicy{AllowDelete: false}
		res, err := execTxnWithRunSQLEventsAndPolicy(
			t, bs, 100, []string{`DELETE FROM foo_1337_100`}, policy)
		require.NoError(t, err)
		require.Contains(t, *res.Error, "delete is not allowed by policy")
	})

	t.Run("update column not-allowed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, _ := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		// set the controller to anything other than zero
		assertExecTxnWithSetController(t, bs, 100, "0x1")
		require.NoError(t, err)

		policy := ethereum.ITablelandControllerPolicy{AllowUpdate: true, UpdatableColumns: []string{"zaz"}}
		// tries to update zar and not zaz
		res, err := execTxnWithRunSQLEventsAndPolicy(
			t, bs, 100, []string{`update foo_1337_100 set zar = 'three';`}, policy)
		require.NoError(t, err)
		require.Contains(t, *res.Error, "column zar is not allowed")
	})

	t.Run("update where policy", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, dbURI := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		// set the controller to anything other than zero
		assertExecTxnWithSetController(t, bs, 100, "0x1")
		require.NoError(t, err)

		// start with two rows
		q := `insert into foo_1337_100 values ('one');`
		q += `insert into foo_1337_100 values ('two');`
		assertExecTxnWithRunSQLEvents(t, bs, 100, []string{q})

		policy := ethereum.ITablelandControllerPolicy{
			AllowUpdate:      true,
			WhereClause:      "zar = 'two'",
			UpdatableColumns: []string{"zar"},
		}
		// send an update that updates all rows with a policy to restricts the update
		res, err := execTxnWithRunSQLEventsAndPolicy(t, bs, 100, []string{`update foo_1337_100 set zar = 'three'`}, policy)
		require.NoError(t, err)
		require.Nil(t, res.Error)
		require.Nil(t, res.ErrorEventIdx)

		require.NoError(t, bs.Commit())
		require.NoError(t, bs.Close())
		require.NoError(t, ex.Close(ctx))

		// there should be only one row updated
		require.Equal(t, 1, tableRowCountT100(t, dbURI, "select count(*) from foo_1337_100 WHERE zar = 'three'"))
	})
}

func TestRunSQL_RowCountLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	rowLimit := 10
	ex, dbURI := newExecutorWithTable(t, rowLimit)

	// Helper func to insert a row and return the result.
	insertRow := func(t *testing.T) *string {
		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		res, err := execTxnWithRunSQLEvents(t, bs, 100, []string{`insert into foo_1337_100 values ('one')`})
		require.NoError(t, err)
		if res.Error == nil {
			require.NoError(t, bs.Commit())
		}
		require.NoError(t, bs.Close())
		return res.Error
	}

	// Insert up to 10 rows should succeed.
	for i := 0; i < rowLimit; i++ {
		require.Nil(t, insertRow(t))
	}
	require.Equal(t, rowLimit, tableRowCountT100(t, dbURI, "select count(*) from foo_1337_100"))

	// The next insert should fail.
	error := insertRow(t)
	require.Contains(t, *error,
		fmt.Sprintf("table maximum row count exceeded (before %d, after %d)", rowLimit, rowLimit+1),
	)

	require.NoError(t, ex.Close(ctx))
}

func TestWithCheck(t *testing.T) {
	t.Parallel()
	t.Run("insert with check not satistifed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, dbURI := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		// set the controller to anything other than zero
		assertExecTxnWithSetController(t, bs, 100, "0x1")

		policy := ethereum.ITablelandControllerPolicy{AllowInsert: true, WithCheck: "zar = 'two'"}
		res, err := execTxnWithRunSQLEventsAndPolicy(t, bs, 100, []string{`insert into foo_1337_100 values ('one')`}, policy)
		require.NoError(t, err)
		require.Contains(t, *res.Error, "number of affected rows 1 does not match auditing count 0")

		require.NoError(t, bs.Commit())
		require.NoError(t, bs.Close())
		require.NoError(t, ex.Close(ctx))

		require.Equal(t, 0, tableRowCountT100(t, dbURI, "select count(*) from foo_1337_100"))
	})

	t.Run("update with check not satistifed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, dbURI := newExecutorWithTable(t, 0)

		{
			bs, err := ex.NewBlockScope(ctx, 0)
			require.NoError(t, err)

			// set the controller to anything other than zero
			assertExecTxnWithSetController(t, bs, 100, "0x1")
			assertExecTxnWithRunSQLEvents(t, bs, 100, []string{`insert into foo_1337_100 values ('one')`})
			require.NoError(t, bs.Commit())
			require.NoError(t, bs.Close())
		}
		{
			bs, err := ex.NewBlockScope(ctx, 0)
			require.NoError(t, err)

			policy := ethereum.ITablelandControllerPolicy{AllowUpdate: true, WithCheck: "zar = 'two'"}
			res, err := execTxnWithRunSQLEventsAndPolicy(t, bs, 100, []string{`update foo_1337_100 SET zar = 'three'`}, policy)
			require.NoError(t, err)
			require.Contains(t, *res.Error, "number of affected rows 1 does not match auditing count 0")

			require.NoError(t, bs.Commit())
			require.NoError(t, bs.Close())
		}
		require.NoError(t, ex.Close(ctx))

		require.Equal(t, 1, tableRowCountT100(t, dbURI, "select count(*) from foo_1337_100 WHERE zar = 'one'"))
		require.Equal(t, 0, tableRowCountT100(t, dbURI, "select count(*) from foo_1337_100 WHERE zar = 'three'"))
	})

	t.Run("insert with check satistifed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, dbURI := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		// set the controller to anything other than zero
		assertExecTxnWithSetController(t, bs, 100, "0x1")

		policy := ethereum.ITablelandControllerPolicy{AllowInsert: true, WithCheck: "zar in ('one', 'two')"}
		q := `insert into foo_1337_100 values ('one');`
		q += `insert into foo_1337_100 values ('two')`
		res, err := execTxnWithRunSQLEventsAndPolicy(t, bs, 100, []string{q}, policy)
		require.NoError(t, err)
		require.Nil(t, res.Error)
		require.Nil(t, res.ErrorEventIdx)

		require.NoError(t, bs.Commit())
		require.NoError(t, bs.Close())
		require.NoError(t, ex.Close(ctx))

		require.Equal(t, 2, tableRowCountT100(t, dbURI, "select count(*) from foo_1337_100"))
	})

	t.Run("row count limit-withcheck", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		rowLimit := 10
		ex, dbURI := newExecutorWithTable(t, rowLimit)

		{
			bs, err := ex.NewBlockScope(ctx, 0)
			require.NoError(t, err)
			// set the controller to anything other than zero
			assertExecTxnWithSetController(t, bs, 100, "0x1")
			require.NoError(t, bs.Close())
		}

		// Helper func to insert a row and return an error if happened.
		insertRow := func(t *testing.T) *string {
			bs, err := ex.NewBlockScope(ctx, 0)
			require.NoError(t, err)

			policy := ethereum.ITablelandControllerPolicy{AllowInsert: true, WithCheck: "zar in ('one')"}
			res, err := execTxnWithRunSQLEventsAndPolicy(t, bs, 100, []string{`insert into foo_1337_100 values ('one')`}, policy)
			require.NoError(t, err)
			if res.Error == nil {
				require.NoError(t, bs.Commit())
			}
			require.NoError(t, bs.Close())
			return res.Error
		}

		// Insert up to 10 rows should succeed.
		for i := 0; i < rowLimit; i++ {
			require.Nil(t, insertRow(t))
		}
		require.Equal(t, rowLimit, tableRowCountT100(t, dbURI, "select count(*) from foo_1337_100"))

		// The next insert should fail.
		error := insertRow(t)
		require.Contains(t, *error,
			fmt.Sprintf("table maximum row count exceeded (before %d, after %d)", rowLimit, rowLimit+1))
		require.NoError(t, ex.Close(ctx))
	})
}

func assertExecTxnWithRunSQLEvents(t *testing.T, bs executor.BlockScope, tableID int, stmts []string) {
	t.Helper()

	res, err := execTxnWithRunSQLEvents(t, bs, tableID, stmts)
	require.NoError(t, err)
	require.NotNil(t, res.TableID)
	require.Equal(t, res.TableID.ToBigInt().Int64(), int64(tableID))
}

func execTxnWithRunSQLEvents(
	t *testing.T,
	bs executor.BlockScope,
	tableID int,
	stmts []string,
) (executor.TxnExecutionResult, error) {
	t.Helper()

	policy := ethereum.ITablelandControllerPolicy{
		AllowInsert:      true,
		AllowUpdate:      true,
		AllowDelete:      true,
		WhereClause:      "",
		WithCheck:        "",
		UpdatableColumns: nil,
	}
	return execTxnWithRunSQLEventsAndPolicy(t, bs, tableID, stmts, policy)
}

func execTxnWithRunSQLEventsAndPolicy(
	t *testing.T,
	bs executor.BlockScope,
	tableID int,
	stmts []string,
	policy ethereum.ITablelandControllerPolicy,
) (executor.TxnExecutionResult, error) {
	t.Helper()

	events := make([]interface{}, len(stmts))
	for i, stmt := range stmts {
		events[i] = &ethereum.ContractRunSQL{
			IsOwner:   true,
			TableId:   big.NewInt(int64(tableID)),
			Statement: stmt,
			Policy:    policy,
		}
	}
	return bs.ExecuteTxnEvents(context.Background(), eventfeed.TxnEvents{Events: events})
}
