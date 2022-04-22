package impl

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
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

// Random address for testing. The value isn't important
// because the ACL is mocked.
var controller = common.HexToAddress("0x07dfFc57AA386D2b239CaBE8993358DF20BAFBE2")

func TestRunSQL(t *testing.T) {
	t.Parallel()
	t.Run("single-query", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		wq1 := mustWriteStmt(t, `insert into foo_100 values ('one')`)
		err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq1}, tableland.DefaultPolicy{})
		require.NoError(t, err)

		require.NoError(t, b.Commit(ctx))
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		require.Equal(t, 1, tableRowCountT100(t, pool, "select count(*) from _100"))
	})

	t.Run("multiple queries", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		{
			wq1 := mustWriteStmt(t, `insert into foo_100 values ('wq1one')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq1}, tableland.DefaultPolicy{})
			require.NoError(t, err)
		}
		{
			wq1 := mustWriteStmt(t, `insert into foo_100 values ('wq1two')`)
			wq2 := mustWriteStmt(t, `insert into foo_100 values ('wq2three')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq1, wq2}, tableland.DefaultPolicy{})
			require.NoError(t, err)
		}
		{
			wq1 := mustWriteStmt(t, `insert into foo_100 values ('wq1four')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq1}, tableland.DefaultPolicy{})
			require.NoError(t, err)
		}

		require.NoError(t, b.Commit(ctx))
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		require.Equal(t, 4, tableRowCountT100(t, pool, "select count(*) from _100"))
	})

	t.Run("multiple with single failure", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		{
			wq1_1 := mustWriteStmt(t, `insert into foo_100 values ('onez')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq1_1}, tableland.DefaultPolicy{})
			require.NoError(t, err)
		}
		{
			wq2_1 := mustWriteStmt(t, `insert into foo_100 values ('twoz')`)
			wq2_2 := mustWriteStmt(t, `insert into foo_101 values ('threez')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq2_1, wq2_2}, tableland.DefaultPolicy{})
			require.Error(t, err)
		}
		{
			wq3_1 := mustWriteStmt(t, `insert into foo_100 values ('fourz')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq3_1}, tableland.DefaultPolicy{})
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
		require.Equal(t, 2, tableRowCountT100(t, pool, "select count(*) from _100"))
	})

	t.Run("with abrupt close", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		{
			wq1_1 := mustWriteStmt(t, `insert into foo_100 values ('one')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq1_1}, tableland.DefaultPolicy{})
			require.NoError(t, err)
		}
		{
			wq2_1 := mustWriteStmt(t, `insert into foo_100 values ('two')`)
			wq2_2 := mustWriteStmt(t, `insert into foo_100 values ('three')`)
			err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq2_1, wq2_2}, tableland.DefaultPolicy{})
			require.NoError(t, err)
		}

		// Note: we don't do a Commit() call, thus all should be rollbacked.
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		// The opened batch wasn't txnp.CloseBatch(), but we simply
		// closed the whole store. This should rollback any ongoing
		// opened batch and leave db state correctly.
		require.Equal(t, 0, tableRowCountT100(t, pool, "select count(*) from _100"))
	})

	t.Run("single-query grant", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		wq1 := mustGrantStmt(t, "grant insert, update, delete on foo_100 to \"0xd43c59d5694ec111eb9e986c233200b14249558d\", \"0x4afe8e30db4549384b0a05bb796468b130c7d6e0\"") //nolint
		err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq1}, tableland.DefaultPolicy{})
		require.NoError(t, err)

		require.NoError(t, b.Commit(ctx))
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		ss := wq1.(parsing.SugaredGrantStmt)
		for _, role := range ss.GetRoles() {
			// Check that an entry was inserted in the system_acl table for each row.
			systemStore, err := system.New(pool)
			require.NoError(t, err)
			aclRow, err := systemStore.GetACLOnTableByController(ctx, ss.GetTableID(), role.String())
			require.NoError(t, err)
			require.Equal(t, wq1.GetTableID(), aclRow.TableID)
			require.Equal(t, role.String(), aclRow.Controller)
			require.Equal(t, ss.GetPrivileges(), aclRow.Privileges)
		}
	})

	t.Run("grant upsert", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		wq1 := mustGrantStmt(t, "grant insert on foo_100 to \"0xd43c59d5694ec111eb9e986c233200b14249558d\", \"0x4afe8e30db4549384b0a05bb796468b130c7d6e0\"") //nolint
		// add the update privilege for role 0xd43c59d5694ec111eb9e986c233200b14249558d
		wq2 := mustGrantStmt(t, "grant update on foo_100 to \"0xd43c59d5694ec111eb9e986c233200b14249558d\"")
		// add the delete privilege (and mistakenly the insert) grant for role 0x4afe8e30db4549384b0a05bb796468b130c7d6e0
		wq3 := mustGrantStmt(t, "grant insert, delete on foo_100 to \"0x4afe8e30db4549384b0a05bb796468b130c7d6e0\"")
		err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq1, wq2, wq3}, tableland.DefaultPolicy{})
		require.NoError(t, err)

		require.NoError(t, b.Commit(ctx))
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		systemStore, err := system.New(pool)
		require.NoError(t, err)

		ss := wq1.(parsing.SugaredGrantStmt)
		{
			aclRow, err := systemStore.GetACLOnTableByController(
				ctx,
				ss.GetTableID(),
				"0xD43C59d5694eC111Eb9e986C233200b14249558D")
			require.NoError(t, err)
			require.Equal(t, wq2.GetTableID(), aclRow.TableID)
			require.Equal(t, "0xD43C59d5694eC111Eb9e986C233200b14249558D", aclRow.Controller)
			require.Equal(t, tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate}, aclRow.Privileges)
		}

		{
			aclRow, err := systemStore.GetACLOnTableByController(
				ctx,
				ss.GetTableID(),
				"0x4afE8e30DB4549384b0a05bb796468B130c7D6E0")
			require.NoError(t, err)
			require.Equal(t, wq3.GetTableID(), aclRow.TableID)
			require.Equal(t, "0x4afE8e30DB4549384b0a05bb796468B130c7D6E0", aclRow.Controller)
			require.Equal(t, tableland.Privileges{tableland.PrivInsert, tableland.PrivDelete}, aclRow.Privileges)
		}
	})

	t.Run("grant revoke", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		wq1 := mustGrantStmt(t, "grant insert, update, delete on foo_100 to \"0xd43c59d5694ec111eb9e986c233200b14249558d\"")
		wq2 := mustGrantStmt(t, "revoke insert, delete on foo_100 from \"0xd43c59d5694ec111eb9e986c233200b14249558d\"")
		err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq1, wq2}, tableland.DefaultPolicy{})
		require.NoError(t, err)

		require.NoError(t, b.Commit(ctx))
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		systemStore, err := system.New(pool)
		require.NoError(t, err)

		ss := wq1.(parsing.SugaredGrantStmt)
		{
			aclRow, err := systemStore.GetACLOnTableByController(
				ctx,
				ss.GetTableID(),
				"0xD43C59d5694eC111Eb9e986C233200b14249558D")
			require.NoError(t, err)
			require.Equal(t, wq2.GetTableID(), aclRow.TableID)
			require.Equal(t, "0xD43C59d5694eC111Eb9e986C233200b14249558D", aclRow.Controller)
			require.Equal(t, tableland.Privileges{tableland.PrivUpdate}, aclRow.Privileges)
		}
	})
}

func TestRunSQLWithPolicies(t *testing.T) {
	t.Parallel()

	t.Run("insert-not-allowed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _ := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		policy := policyFactory(policyData{
			isInsertAllowed: false,
		})

		wq := mustWriteStmt(t, `insert into foo_100 values ('one');`)
		err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq}, policy)
		require.Error(t, err)
	})

	t.Run("update-not-allowed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _ := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		policy := policyFactory(policyData{
			isUpdateAllowed: false,
		})

		wq := mustWriteStmt(t, `update foo_100 set zar = 'three';`)
		err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq}, policy)
		require.Error(t, err)
	})

	t.Run("delete-not-allowed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _ := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		policy := policyFactory(policyData{
			isDeleteAllowed: false,
		})

		wq := mustWriteStmt(t, `DELETE FROM foo_100`)
		err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq}, policy)
		require.Error(t, err)
	})

	t.Run("update-column-not-allowed", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, _ := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		policy := policyFactory(policyData{
			isUpdateAllowed: true,
			updateColumns:   []string{"zaz"}, // zaz instead of zar
		})

		// tries to update zar and not zaz
		wq := mustWriteStmt(t, `update foo_100 set zar = 'three';`)
		err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq}, policy)
		require.ErrorContains(t, err, "column zar is not allowed")
	})

	t.Run("update-where-policy", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, pool := newTxnProcessorWithTable(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		// start with two rows
		wq1 := mustWriteStmt(t, `insert into foo_100 values ('one');`)
		wq2 := mustWriteStmt(t, `insert into foo_100 values ('two');`)
		err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq1, wq2}, tableland.DefaultPolicy{})
		require.NoError(t, err)

		policy := policyFactory(policyData{
			isUpdateAllowed: true,
			updateWhere:     "zar = 'two'",
			updateColumns:   []string{"zar"},
		})

		// send an update that updates all rows with a policy to restrics the update
		wq3 := mustWriteStmt(t, `update foo_100 set zar = 'three'`)
		err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{wq3}, policy)
		require.NoError(t, err)

		require.NoError(t, b.Commit(ctx))
		require.NoError(t, b.Close(ctx))
		require.NoError(t, txnp.Close(ctx))

		// there should be only one row updated
		require.Equal(t, 1, tableRowCountT100(t, pool, "select count(*) from _100 WHERE zar = 'three'"))
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
		err = b.InsertTable(ctx, id, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", createStmt)
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
		err = b.ExecWriteQueries(ctx, controller, []parsing.SugaredMutatingStmt{q}, tableland.DefaultPolicy{})
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
	require.Equal(t, rowLimit, tableRowCountT100(t, pool, "select count(*) from _100"))

	// The next insert should fail.
	var errRowCount *txn.ErrRowCountExceeded
	require.ErrorAs(t, insertRow(t), &errRowCount)
	require.Equal(t, rowLimit, errRowCount.BeforeRowCount)
	require.Equal(t, rowLimit+1, errRowCount.AfterRowCount)

	require.NoError(t, txnp.Close(ctx))
}

func tableRowCountT100(t *testing.T, pool *pgxpool.Pool, sql string) int {
	t.Helper()

	row := pool.QueryRow(context.Background(), sql)
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

	url := tests.PostgresURL(t)

	txnp, err := NewTxnProcessor(url, rowsLimit, &aclMock{})
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
	err = b.InsertTable(ctx, id, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", createStmt)
	require.NoError(t, err)

	require.NoError(t, b.Commit(ctx))
	require.NoError(t, b.Close(ctx))

	return txnp, pool
}

func mustWriteStmt(t *testing.T, q string) parsing.SugaredMutatingStmt {
	t.Helper()
	p := parserimpl.New([]string{"system_", "registry"}, 0, 0)
	_, wss, err := p.ValidateRunSQL(q)
	require.NoError(t, err)
	require.Len(t, wss, 1)
	return wss[0]
}

func mustGrantStmt(t *testing.T, q string) parsing.SugaredMutatingStmt {
	t.Helper()
	p := parserimpl.New([]string{"system_", "registry"}, 0, 0)
	_, wss, err := p.ValidateRunSQL(q)
	require.NoError(t, err)
	require.Len(t, wss, 1)
	return wss[0]
}

type aclMock struct{}

func (acl *aclMock) CheckPrivileges(
	ctx context.Context,
	tx pgx.Tx,
	controller common.Address,
	id tableland.TableID,
	op tableland.Operation) (bool, error) {
	return true, nil
}

func (acl *aclMock) IsOwner(ctx context.Context, controller common.Address, id tableland.TableID) (bool, error) {
	return true, nil
}

type policyData struct {
	isInsertAllowed bool
	isUpdateAllowed bool
	isDeleteAllowed bool
	updateWhere     string
	updateColumns   []string
}

func policyFactory(data policyData) tableland.Policy {
	return policy{data}
}

type policy struct {
	policyData
}

func (p policy) IsInsertAllowed() bool   { return p.policyData.isInsertAllowed }
func (p policy) IsUpdateAllowed() bool   { return p.policyData.isUpdateAllowed }
func (p policy) IsDeleteAllowed() bool   { return p.policyData.isDeleteAllowed }
func (p policy) UpdateWhere() string     { return p.policyData.updateWhere }
func (p policy) UpdateColumns() []string { return p.policyData.updateColumns }
