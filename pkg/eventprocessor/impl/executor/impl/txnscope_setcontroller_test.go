package impl

import (
	"context"
	"database/sql"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
)

func TestSetController(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("controller is not set default", func(t *testing.T) {
		t.Parallel()
		_, _, db := newExecutorWithTable(t, 0)

		// Let's test first that the controller is not set (it's the default behavior)
		controller := getController(t, db, 100)
		require.Equal(t, "", controller)
	})

	t.Run("foreign key constraint", func(t *testing.T) {
		t.Parallel()
		ex, _, _ := newExecutorWithTable(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)
		// table id different than 100 violates foreign key
		res, err := execTxnWithSetController(t, bs, 1, "0x1")
		require.NoError(t, err)
		require.Contains(t, *res.Error, "FOREIGN KEY constraint failed")

		require.NoError(t, bs.Commit())
		require.NoError(t, bs.Close())
		require.NoError(t, ex.Close(ctx))
	})

	t.Run("set unset controller", func(t *testing.T) {
		t.Parallel()
		ex, _, db := newExecutorWithTable(t, 0)

		// sets
		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		assertExecTxnWithSetController(t, bs, 100, "0x01")
		require.NoError(t, bs.Commit())
		require.NoError(t, bs.Close())

		controller := getController(t, db, 100)
		require.Equal(t, "0x0000000000000000000000000000000000000001", controller)

		// unsets
		bs, err = ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)
		assertExecTxnWithSetController(t, bs, 100, "0x0")
		require.NoError(t, bs.Commit())
		require.NoError(t, bs.Close())

		controller = getController(t, db, 100)
		require.Equal(t, "", controller)

		require.NoError(t, ex.Close(ctx))
	})

	t.Run("upsert", func(t *testing.T) {
		t.Parallel()
		ex, _, db := newExecutorWithTable(t, 0)

		{
			bs, err := ex.NewBlockScope(ctx, 0)
			require.NoError(t, err)
			assertExecTxnWithSetController(t, bs, 100, "0x01")
			require.NoError(t, bs.Commit())
			require.NoError(t, bs.Close())
		}

		{
			bs, err := ex.NewBlockScope(ctx, 0)
			require.NoError(t, err)
			assertExecTxnWithSetController(t, bs, 100, "0x02")
			require.NoError(t, bs.Commit())
			require.NoError(t, bs.Close())
		}

		controller := getController(t, db, 100)
		require.Equal(t, "0x0000000000000000000000000000000000000002", controller)

		require.NoError(t, ex.Close(ctx))
	})
}

func getController(t *testing.T, db *sql.DB, tableID int64) string {
	q := "SELECT controller FROM system_controller where chain_id=1337 AND table_id=?1"
	r := db.QueryRowContext(context.Background(), q, tableID)
	var controller string
	err := r.Scan(&controller)
	if err == sql.ErrNoRows {
		return ""
	}
	require.NoError(t, err)
	return controller
}

func assertExecTxnWithSetController(t *testing.T, bs executor.BlockScope, tableID int, controller string) {
	t.Helper()

	res, err := execTxnWithSetController(t, bs, tableID, controller)
	require.NoError(t, err)
	require.NotNil(t, res.TableID)
	require.Equal(t, res.TableID.ToBigInt().Int64(), int64(tableID))
}

func execTxnWithSetController(t *testing.T, bs executor.BlockScope, tableID int, controller string) (executor.TxnExecutionResult, error) {
	t.Helper()

	e := &ethereum.ContractSetController{
		TableId:    big.NewInt(int64(tableID)),
		Controller: common.HexToAddress(controller),
	}
	return bs.ExecuteTxnEvents(context.Background(), eventfeed.TxnEvents{Events: []interface{}{e}})
}