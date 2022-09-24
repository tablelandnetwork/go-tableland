package counterprobe

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/mocks"
	"github.com/textileio/go-tableland/pkg/client"
	"github.com/textileio/go-tableland/pkg/testutils"
	"github.com/textileio/go-tableland/pkg/wallet"
)

func TestProbe(t *testing.T) {
	txn := mocks.NewTransaction(t)
	txn.EXPECT().Hash().Return(common.BigToHash(big.NewInt(1)))

	tbl := mocks.NewTableland(t)
	tbl.EXPECT().RunReadQuery(mock.Anything, mock.AnythingOfType("string")).Return(
		&tableland.TableData{
			Columns: []tableland.Column{{Name: "counter"}},
			Rows:    [][]*tableland.ColumnValue{{tableland.OtherColValue(1)}},
		},
		nil,
	).Once()
	tbl.EXPECT().RelayWriteQuery(mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("string")).Return(
		txn,
		nil,
	).Once()
	tbl.EXPECT().GetReceipt(mock.Anything, mock.Anything, txn.Hash().Hex()).Return(
		true,
		&tableland.TxnReceipt{
			TxnHash: txn.Hash().Hex(),
		},
		nil,
	).Once()
	tbl.EXPECT().RunReadQuery(mock.Anything, mock.AnythingOfType("string")).Return(
		&tableland.TableData{
			Columns: []tableland.Column{{Name: "counter"}},
			Rows:    [][]*tableland.ColumnValue{{tableland.OtherColValue(2)}},
		},
		nil,
	).Once()

	stack := testutils.CreateFullStack(t, testutils.Deps{Tableland: tbl})

	cp, err := New("optimism-mainnet", stack.Client, "Runbook_24", time.Second, time.Second*10)
	require.NoError(t, err)

	value, err := cp.healthCheck(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), value)
}

func TestProduction(t *testing.T) {
	t.SkipNow()
	endpoint := "https://testnet.tableland.network/rpc"
	pk := "fillme"
	wallet, err := wallet.NewWallet(pk)
	require.NoError(t, err)
	tblname := "Runbook_24"

	chain := client.Chain{Endpoint: endpoint}

	client, err := client.NewClient(
		context.Background(),
		wallet,
		client.NewClientChain(chain),
		client.NewClientRelayWrites(true),
	)
	require.NoError(t, err)

	cp, err := New("optimism-mainnet", client, tblname, time.Second, time.Second*10)
	require.NoError(t, err)

	value, err := cp.healthCheck(context.Background())
	require.NoError(t, err)
	require.NotZero(t, value)
}
