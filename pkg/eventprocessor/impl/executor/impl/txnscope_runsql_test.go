package impl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
)

func TestRunSQL_OneEvent(t *testing.T) {
	t.Parallel()
	t.Run("one insert", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		ex, _, pool := newExecutorWithTable(t, 0)

		b, err := ex.NewBlockScope(ctx, 0, "")
		require.NoError(t, err)

		wq1 := mustWriteStmt(t, `insert into foo_1337_100 values ('one')`)
		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq1}, true, tableland.AllowAllPolicy{})
		require.NoError(t, err)

		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, ex.Close(ctx))

		require.Equal(t, 1, tableRowCountT100(t, pool, "select count(*) from foo_1337_100"))
	})

	t.Run("multiple inserts", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _, pool := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		{
			wq1 := mustWriteStmt(t, `insert into foo_1337_100 values ('wq1one')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq1}, true, tableland.AllowAllPolicy{})
			require.NoError(t, err)
		}
		{
			wq1 := mustWriteStmt(t, `insert into foo_1337_100 values ('wq1two')`)
			wq2 := mustWriteStmt(t, `insert into foo_1337_100 values ('wq2three')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq1, wq2}, true, tableland.AllowAllPolicy{})
			require.NoError(t, err)
		}
		{
			wq1 := mustWriteStmt(t, `insert into foo_1337_100 values ('wq1four')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq1}, true, tableland.AllowAllPolicy{})
			require.NoError(t, err)
		}

		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, txnp.Close(ctx))

		require.Equal(t, 4, tableRowCountT100(t, pool, "select count(*) from foo_1337_100"))
	})

	t.Run("multiple with single failure", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _, pool := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		{
			wq1_1 := mustWriteStmt(t, `insert into foo_1337_100 values ('onez')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq1_1}, true, tableland.AllowAllPolicy{})
			require.NoError(t, err)
		}
		{
			wq2_1 := mustWriteStmt(t, `insert into foo_1337_100 values ('twoz')`)
			wq2_2 := mustWriteStmt(t, `insert into foo_1337_101 values ('threez')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq2_1, wq2_2}, true, tableland.AllowAllPolicy{}) // nolint
			require.Error(t, err)
		}
		{
			wq3_1 := mustWriteStmt(t, `insert into foo_1337_100 values ('fourz')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq3_1}, true, tableland.AllowAllPolicy{})
			require.NoError(t, err)
		}

		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, txnp.Close(ctx))

		// We executed a single batch, with 3 Exec calls.
		// The second Exec should have failed as a whole.
		//
		// Note that its wq2_1 succeeded, but wq2_2 failed, this means:
		// 1. wq1_1 and wq3_1 should survive the whole batch commit.
		// 2. despite wq2_1 apparently should succeed, wq2_2 failure should rollback
		//    both wq2_* statements.
		require.Equal(t, 2, tableRowCountT100(t, pool, "select count(*) from foo_1337_100"))
	})

	t.Run("with abrupt close", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _, pool := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		{
			wq1_1 := mustWriteStmt(t, `insert into foo_1337_100 values ('one')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq1_1}, true, tableland.AllowAllPolicy{})
			require.NoError(t, err)
		}
		{
			wq2_1 := mustWriteStmt(t, `insert into foo_1337_100 values ('two')`)
			wq2_2 := mustWriteStmt(t, `insert into foo_1337_100 values ('three')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq2_1, wq2_2}, true, tableland.AllowAllPolicy{}) // nolint
			require.NoError(t, err)
		}

		// Note: we don't do a Commit() call, thus all should be rollbacked.
		require.NoError(t, b.Close())
		require.NoError(t, txnp.Close(ctx))

		// The opened batch wasn't txnp.CloseBatch(), but we simply
		// closed the whole store. This should rollback any ongoing
		// opened batch and leave db state correctly.
		require.Equal(t, 0, tableRowCountT100(t, pool, "select count(*) from foo_1337_100"))
	})

	t.Run("single-query grant", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, dbURL, _ := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		wq1 := mustGrantStmt(t, "grant insert, update, delete on foo_1337_100 to '0xd43c59d5694ec111eb9e986c233200b14249558d', '0x4afe8e30db4549384b0a05bb796468b130c7d6e0'") // nolint
		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq1}, true, tableland.AllowAllPolicy{})
		require.NoError(t, err)

		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, txnp.Close(ctx))

		ss := wq1.(parsing.GrantStmt)
		for _, role := range ss.GetRoles() {
			// Check that an entry was inserted in the system_acl table for each row.
			systemStore, err := system.New(dbURL, tableland.ChainID(chainID))
			require.NoError(t, err)
			aclRow, err := systemStore.GetACLOnTableByController(ctx, ss.GetTableID(), role.String())
			require.NoError(t, err)
			require.Equal(t, wq1.GetTableID(), aclRow.TableID)
			require.Equal(t, role.String(), aclRow.Controller)
			require.ElementsMatch(t, ss.GetPrivileges(), aclRow.Privileges)
		}
	})

	t.Run("grant upsert", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, dbURL, _ := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		wq1 := mustGrantStmt(t, "grant insert on foo_1337_100 to '0xd43c59d5694ec111eb9e986c233200b14249558d', '0x4afe8e30db4549384b0a05bb796468b130c7d6e0'") // nolint
		// add the update privilege for role 0xd43c59d5694ec111eb9e986c233200b14249558d
		wq2 := mustGrantStmt(t, "grant update on foo_1337_100 to '0xd43c59d5694ec111eb9e986c233200b14249558d'")
		// add the delete privilege (and mistakenly the insert) grant for role 0x4afe8e30db4549384b0a05bb796468b130c7d6e0
		wq3 := mustGrantStmt(t, "grant insert, delete on foo_1337_100 to '0x4afe8e30db4549384b0a05bb796468b130c7d6e0'")
		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq1, wq2, wq3}, true, tableland.AllowAllPolicy{}) // nolint
		require.NoError(t, err)

		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, txnp.Close(ctx))

		systemStore, err := system.New(dbURL, tableland.ChainID(chainID))
		require.NoError(t, err)

		ss := wq1.(parsing.GrantStmt)
		{
			aclRow, err := systemStore.GetACLOnTableByController(
				ctx,
				ss.GetTableID(),
				"0xD43C59d5694eC111Eb9e986C233200b14249558D")
			require.NoError(t, err)
			require.Equal(t, wq2.GetTableID(), aclRow.TableID)
			require.Equal(t, "0xD43C59d5694eC111Eb9e986C233200b14249558D", aclRow.Controller)
			require.ElementsMatch(t, tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate}, aclRow.Privileges)
		}

		{
			aclRow, err := systemStore.GetACLOnTableByController(
				ctx,
				ss.GetTableID(),
				"0x4afE8e30DB4549384b0a05bb796468B130c7D6E0")
			require.NoError(t, err)
			require.Equal(t, wq3.GetTableID(), aclRow.TableID)
			require.Equal(t, "0x4afE8e30DB4549384b0a05bb796468B130c7D6E0", aclRow.Controller)
			require.ElementsMatch(t, tableland.Privileges{tableland.PrivInsert, tableland.PrivDelete}, aclRow.Privileges)
		}
	})

	t.Run("grant revoke", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, dbURL, _ := newExecutorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		wq1 := mustGrantStmt(t, "grant insert, update, delete on foo_1337_100 to '0xd43c59d5694ec111eb9e986c233200b14249558d'") // nolint
		wq2 := mustGrantStmt(t, "revoke insert, delete on foo_1337_100 from '0xd43c59d5694ec111eb9e986c233200b14249558d'")
		err = b.ExecWriteQueries(ctx, controller, []parsing.MutatingStmt{wq1, wq2}, true, tableland.AllowAllPolicy{})
		require.NoError(t, err)

		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, txnp.Close(ctx))

		systemStore, err := system.New(dbURL, tableland.ChainID(chainID))
		require.NoError(t, err)

		ss := wq1.(parsing.GrantStmt)
		{
			aclRow, err := systemStore.GetACLOnTableByController(
				ctx,
				ss.GetTableID(),
				"0xD43C59d5694eC111Eb9e986C233200b14249558D")
			require.NoError(t, err)
			require.Equal(t, wq2.GetTableID(), aclRow.TableID)
			require.Equal(t, "0xD43C59d5694eC111Eb9e986C233200b14249558D", aclRow.Controller)
			require.ElementsMatch(t, tableland.Privileges{tableland.PrivUpdate}, aclRow.Privileges)
		}
	})
}
