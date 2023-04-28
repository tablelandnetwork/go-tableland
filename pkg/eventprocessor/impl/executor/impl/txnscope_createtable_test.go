package impl

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	gatewayimpl "github.com/textileio/go-tableland/internal/gateway/impl"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
)

func TestCreateTable(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		ex, dbURI := newExecutor(t, 0)

		bs, err := ex.NewBlockScope(ctx, 0)
		require.NoError(t, err)

		assertExecTxnWithCreateTable(t, bs, 100, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", "create table bar_1337 (zar text)") //nolint

		require.NoError(t, bs.Commit())
		require.NoError(t, bs.Close())
		require.NoError(t, ex.Close(ctx))

		// Check that the table was registered in the system-table.

		tableID, _ := tables.NewTableID("100")
		table, err := gatewayimpl.NewGatewayStore(ex.db, nil).GetTable(ctx, 1337, tableID)
		require.NoError(t, err)
		require.Equal(t, tableID, table.ID)
		require.Equal(t, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", table.Controller)
		// echo -n zar:TEXT | shasum -a 256
		require.Equal(t, "7ec5320c16e06e90af5e7131ff0c80d4b0a08fcd62aa6e38ad8d6843bc480d09", table.Structure)
		require.Equal(t, "bar", table.Prefix)
		require.NotEqual(t, new(time.Time), table.CreatedAt) // CreatedAt is not the zero value

		// Check that the user table was created.
		ok := existsTableWithName(t, dbURI, "bar_1337_100")
		require.True(t, ok)
	})
}

func assertExecTxnWithCreateTable(t *testing.T, bs executor.BlockScope, tableID int, owner string, stmt string) {
	t.Helper()

	e := &ethereum.ContractCreateTable{
		TableId:   big.NewInt(int64(tableID)),
		Owner:     common.HexToAddress(owner),
		Statement: stmt,
	}
	res, err := bs.ExecuteTxnEvents(context.Background(), eventfeed.TxnEvents{Events: []interface{}{e}})
	require.NoError(t, err)
	require.NotNil(t, res.TableID)
	require.Equal(t, res.TableID.ToBigInt().Int64(), int64(tableID))
}
