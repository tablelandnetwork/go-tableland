package impl

import (
	"context"
	"database/sql"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor"
)

func TestSetController(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tableID := tableland.TableID(*big.NewInt(100))

	t.Run("controller is not set default", func(t *testing.T) {
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

	t.Run("foreign key constraint", func(t *testing.T) {
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

	t.Run("set unset controller", func(t *testing.T) {
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
