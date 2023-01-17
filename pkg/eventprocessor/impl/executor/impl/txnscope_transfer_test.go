package impl

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
)

func TestTransfer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ex, dbURI := newExecutorWithIntegerTable(t, 0)

	require.Equal(t, 1,
		tableReadInteger(
			t,
			dbURI,
			fmt.Sprintf(
				"select count(1) from registry WHERE controller = '%s' and id = %d and chain_id = %d",
				"0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF",
				100,
				chainID,
			),
		))

	bs, err := ex.NewBlockScope(ctx, 0)
	require.NoError(t, err)

	// change table's owner
	newOwner := "0x07dfFc57AA386D2b239CaBE8993358DF20BAFBE2"
	assertExecTxnWithTransfer(t, bs, 100, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", newOwner)
	require.NoError(t, err)
	require.NoError(t, bs.Commit())
	require.NoError(t, bs.Close())
	require.NoError(t, ex.Close(ctx))

	require.Equal(t, 1,
		tableReadInteger(
			t,
			dbURI,
			fmt.Sprintf(
				"select count(1) from registry WHERE controller = '%s' and id = %d and chain_id = %d",
				newOwner,
				100,
				chainID,
			),
		))
}

func assertExecTxnWithTransfer(t *testing.T, bs executor.BlockScope, tableID int, from string, to string) {
	t.Helper()

	e := &ethereum.ContractTransferTable{
		TableId: big.NewInt(int64(tableID)),
		From:    common.HexToAddress(from),
		To:      common.HexToAddress(to),
	}
	res, err := bs.ExecuteTxnEvents(context.Background(), eventfeed.TxnEvents{Events: []interface{}{e}})
	require.NoError(t, err)
	require.NotNil(t, res.TableID)
	require.Equal(t, res.TableID.ToBigInt().Int64(), int64(tableID))
}
