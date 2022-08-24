package impl

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/tables/impl/testutil"
	"github.com/textileio/go-tableland/tests"
)

var emptyHash = common.HexToHash("0x0")

func TestRunSQLEvents(t *testing.T) {
	t.Parallel()

	dbURI := tests.Sqlite3URI()
	systemStore, err := system.New(dbURI, tableland.ChainID(1337))
	require.NoError(t, err)

	backend, addr, sc, authOpts, _ := testutil.Setup(t)
	ef, err := New(systemStore, 1337, backend, addr, eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)

	// Create the table
	ctrl := authOpts.From
	_, err = sc.CreateTable(
		authOpts,
		ctrl,
		"CREATE TABLE foo (bar int)")
	require.NoError(t, err)

	// Make one call before start listening.
	_, err = sc.RunSQL(authOpts, ctrl, big.NewInt(1), "stmt-1")
	require.NoError(t, err)
	backend.Commit()

	// Start listening to Logs for the contract from the next block.
	currBlockNumber := backend.Blockchain().CurrentHeader().Number.Int64()
	ch := make(chan eventfeed.BlockEvents)
	go func() {
		err := ef.Start(context.Background(), currBlockNumber+1, ch, []eventfeed.EventType{eventfeed.RunSQL})
		require.NoError(t, err)
	}()

	// Verify that the RunSQL call we did before listening doesn't appear,
	// since we start from height currentBlockNumber+1 intentionally excluding
	// the first runSQL call. This is to check that the `fromHeight` argument
	// in Start(...) is working as expected.
	select {
	case <-ch:
		t.Fatalf("received unexpected event")
	default:
	}

	// Make a second call, that should be detected as a new event next.
	_, err = sc.RunSQL(authOpts, ctrl, big.NewInt(1), "stmt-2")
	require.NoError(t, err)
	backend.Commit()
	select {
	case bes := <-ch:
		require.Len(t, bes.Txns, 1)
		require.NotEqual(t, emptyHash, bes.Txns[0].TxnHash)
		require.IsType(t, &ethereum.ContractRunSQL{}, bes.Txns[0].Events[0])
	case <-time.After(time.Second):
		t.Fatalf("didn't receive expected log")
	}

	// Try making two calls in a single block now, and assert we receive things correctly.
	_, err = sc.RunSQL(authOpts, ctrl, big.NewInt(1), "stmt-3")
	require.NoError(t, err)
	_, err = sc.RunSQL(authOpts, ctrl, big.NewInt(1), "stmt-4")

	require.NoError(t, err)
	backend.Commit()
	select {
	case bes := <-ch:
		require.Len(t, bes.Txns, 2)
		require.NotEqual(t, emptyHash, bes.Txns[0].TxnHash)
		require.NotEqual(t, emptyHash, bes.Txns[1].TxnHash)
		require.IsType(t, &ethereum.ContractRunSQL{}, bes.Txns[0].Events[0])
		require.IsType(t, &ethereum.ContractRunSQL{}, bes.Txns[1].Events[0])
	case <-time.After(time.Second):
		t.Fatalf("didn't receive expected log")
	}
}

func TestAllEvents(t *testing.T) {
	t.Parallel()

	dbURI := tests.Sqlite3URI()
	systemStore, err := system.New(dbURI, tableland.ChainID(1337))
	require.NoError(t, err)

	backend, addr, sc, authOpts, _ := testutil.Setup(t)
	fetchBlockExtraInfoDelay = time.Millisecond
	ef, err := New(
		systemStore,
		1337,
		backend,
		addr,
		eventfeed.WithMinBlockDepth(0),
		eventfeed.WithEventPersistence(true),
		eventfeed.WithFetchExtraBlockInformation(true))
	require.NoError(t, err)

	ctx, cls := context.WithCancel(context.Background())
	defer cls()
	chFeedClosed := make(chan struct{})
	ch := make(chan eventfeed.BlockEvents)
	go func() {
		err := ef.Start(ctx, 0, ch, []eventfeed.EventType{
			eventfeed.RunSQL,
			eventfeed.CreateTable,
			eventfeed.SetController,
			eventfeed.TransferTable,
		})
		require.NoError(t, err)
		close(chFeedClosed)
	}()

	// Check that there's no enhanced information for the first 10 blocks.
	// 10 is an arbitrary choice to make it future proof if the setup stage decides to mine
	// some extra blocks, so we make sure we're 100% clean.
	for i := int64(0); i < 10; i++ {
		_, err = ef.systemStore.GetBlockExtraInfo(ctx, i)
		require.Error(t, err)
	}

	ctrl := authOpts.From
	// Make four calls to different functions emitting different events
	txn1, err := sc.CreateTable(
		authOpts,
		ctrl,
		"CREATE TABLE foo (bar int)")
	require.NoError(t, err)

	txn2, err := sc.RunSQL(authOpts, ctrl, big.NewInt(1), "stmt-2")
	require.NoError(t, err)

	txn3, err := sc.SetController(
		authOpts,
		ctrl,
		big.NewInt(1),
		common.HexToAddress("0xB0Cf943Cf94E7B6A2657D15af41c5E06c2BFEA3E"),
	)
	require.NoError(t, err)

	txn4, err := sc.TransferFrom(
		authOpts,
		ctrl,
		common.HexToAddress("0xB0Cf943Cf94E7B6A2657D15af41c5E06c2BFEA3E"),
		big.NewInt(1),
	)
	require.NoError(t, err)
	backend.Commit()

	select {
	case bes := <-ch:
		require.Len(t, bes.Txns, 4)

		// Txn1
		{
			require.NotEqual(t, emptyHash, bes.Txns[0].TxnHash)
			require.IsType(t, &ethereum.ContractCreateTable{}, bes.Txns[0].Events[0])

			evmEvents, err := systemStore.GetEVMEvents(ctx, bes.Txns[0].TxnHash)
			require.NoError(t, err)
			evmEvent := evmEvents[0]

			require.Equal(t, txn1.ChainId().Int64(), int64(evmEvent.ChainID))
			require.NotEmpty(t, evmEvent.EventJSON)
			require.Equal(t, "ContractCreateTable", evmEvent.EventType)
			require.Equal(t, *txn1.To(), evmEvent.Address)
			require.NotEmpty(t, evmEvent.Topics)
			require.NotEmpty(t, txn1.Data(), evmEvent.Data)
			require.Equal(t, bes.BlockNumber, int64(evmEvent.BlockNumber))
			require.Equal(t, txn1.Hash(), evmEvent.TxHash)
			require.Equal(t, uint(0), evmEvent.TxIndex)
			require.NotEmpty(t, evmEvent.BlockHash)
			require.Equal(t, uint(1), evmEvent.Index)
		}

		// Txn2
		{
			require.NotEqual(t, emptyHash, bes.Txns[1].TxnHash)
			require.IsType(t, &ethereum.ContractRunSQL{}, bes.Txns[1].Events[0])

			evmEvents, err := systemStore.GetEVMEvents(ctx, bes.Txns[1].TxnHash)
			require.NoError(t, err)
			evmEvent := evmEvents[0]

			require.Equal(t, txn2.ChainId().Int64(), int64(evmEvent.ChainID))
			require.NotEmpty(t, evmEvent.EventJSON)
			require.Equal(t, "ContractRunSQL", evmEvent.EventType)
			require.Equal(t, *txn2.To(), evmEvent.Address)
			require.NotEmpty(t, evmEvent.Topics)
			require.NotEmpty(t, txn2.Data(), evmEvent.Data)
			require.Equal(t, bes.BlockNumber, int64(evmEvent.BlockNumber))
			require.Equal(t, txn2.Hash(), evmEvent.TxHash)
			require.Equal(t, uint(1), evmEvent.TxIndex)
			require.NotEmpty(t, evmEvent.BlockHash)
			require.Equal(t, uint(2), evmEvent.Index)
		}

		// Txn3
		{
			require.IsType(t, &ethereum.ContractSetController{}, bes.Txns[2].Events[0])

			evmEvents, err := systemStore.GetEVMEvents(ctx, bes.Txns[2].TxnHash)
			require.NoError(t, err)
			evmEvent := evmEvents[0]

			require.Equal(t, txn3.ChainId().Int64(), int64(evmEvent.ChainID))
			require.NotEmpty(t, evmEvent.EventJSON)
			require.Equal(t, "ContractSetController", evmEvent.EventType)
			require.Equal(t, *txn3.To(), evmEvent.Address)
			require.NotEmpty(t, evmEvent.Topics)
			require.NotEmpty(t, txn3.Data(), evmEvent.Data)
			require.Equal(t, bes.BlockNumber, int64(evmEvent.BlockNumber))
			require.Equal(t, txn3.Hash(), evmEvent.TxHash)
			require.Equal(t, uint(2), evmEvent.TxIndex)
			require.NotEmpty(t, evmEvent.BlockHash)
			require.Equal(t, uint(3), evmEvent.Index)
		}

		// Txn4
		{
			require.IsType(t, &ethereum.ContractTransferTable{}, bes.Txns[3].Events[0])

			evmEvents, err := systemStore.GetEVMEvents(ctx, bes.Txns[3].TxnHash)
			require.NoError(t, err)
			evmEvent := evmEvents[0]

			require.Equal(t, txn4.ChainId().Int64(), int64(evmEvent.ChainID))
			require.NotEmpty(t, evmEvent.EventJSON)
			require.Equal(t, "ContractTransferTable", evmEvent.EventType)
			require.Equal(t, *txn4.To(), evmEvent.Address)
			require.NotEmpty(t, evmEvent.Topics)
			require.NotEmpty(t, txn4.Data(), evmEvent.Data)
			require.Equal(t, bes.BlockNumber, int64(evmEvent.BlockNumber))
			require.Equal(t, txn4.Hash(), evmEvent.TxHash)
			require.Equal(t, uint(3), evmEvent.TxIndex)
			require.NotEmpty(t, evmEvent.BlockHash)
			require.Equal(t, uint(5), evmEvent.Index)
		}

		var bi tableland.EVMBlockInfo
		require.Eventually(t, func() bool {
			bi, err = ef.systemStore.GetBlockExtraInfo(ctx, bes.BlockNumber)
			return err == nil
		}, time.Second*10, time.Second)
		require.Equal(t, txn1.ChainId().Int64(), int64(bi.ChainID))
		require.Equal(t, bes.BlockNumber, bi.BlockNumber)
		require.NotZero(t, time.Since(bi.Timestamp).Seconds())

	case <-time.After(time.Second):
		t.Fatalf("didn't receive expected log")
	}

	// Test graceful closing.
	cls()
	<-chFeedClosed
}

func TestInfura(t *testing.T) {
	t.Parallel()
	t.SkipNow()

	infuraAPI := os.Getenv("INFURA_API")
	if infuraAPI == "" {
		t.Skipf("no infura API present in env INFURA_API")
	}
	conn, err := ethclient.Dial(infuraAPI)
	require.NoError(t, err)
	rinkebyContractAddr := common.HexToAddress("0x847645b7dAA32eFda757d3c10f1c82BFbB7b41D0")

	dbURI := tests.Sqlite3URI()
	systemStore, err := system.New(dbURI, tableland.ChainID(1337))
	require.NoError(t, err)
	ef, err := New(systemStore, 1337, conn, rinkebyContractAddr, eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)

	ctx, cls := context.WithCancel(context.Background())
	defer cls()
	chFeedClosed := make(chan struct{})
	ch := make(chan eventfeed.BlockEvents)
	go func() {
		contractDeploymentBlockNumber := 10140812 - 100
		err := ef.Start(ctx,
			int64(contractDeploymentBlockNumber),
			ch,
			[]eventfeed.EventType{
				eventfeed.RunSQL,
				eventfeed.CreateTable,
			})
		require.NoError(t, err)
		close(chFeedClosed)
	}()

	var num int
	for {
		select {
		case e := <-ch:
			ct := e.Txns[0].Events[0].(*ethereum.ContractTransfer)
			fmt.Printf("blocknumber %d, %d events. (tokenId %d -> %s)\n", e.BlockNumber, len(e.Txns), ct.TokenId, ct.To)
			num++
			if num > 40 {
				cls()
			}
		case <-chFeedClosed:
			return
		}
	}
}
