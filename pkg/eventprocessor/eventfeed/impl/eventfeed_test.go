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
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/tables/impl/testutil"
)

var emptyHash = common.HexToHash("0x0")

func TestRunSQLEvents(t *testing.T) {
	t.Parallel()

	backend, addr, sc, authOpts, _ := testutil.Setup(t)
	qf, err := New(1337, backend, addr, eventfeed.WithMinBlockDepth(0))
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
		err := qf.Start(context.Background(), currBlockNumber+1, ch, []eventfeed.EventType{eventfeed.RunSQL})
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
		require.Len(t, bes.TxnEvents, 1)
		require.NotEqual(t, emptyHash, bes.TxnEvents[0].TxnHash)
		require.IsType(t, &ethereum.ContractRunSQL{}, bes.TxnEvents[0].Events)
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
		require.Len(t, bes.TxnEvents, 2)
		require.NotEqual(t, emptyHash, bes.TxnEvents[0].TxnHash)
		require.NotEqual(t, emptyHash, bes.TxnEvents[1].TxnHash)
		require.IsType(t, &ethereum.ContractRunSQL{}, bes.TxnEvents[0].Events)
		require.IsType(t, &ethereum.ContractRunSQL{}, bes.TxnEvents[1].Events)
	case <-time.After(time.Second):
		t.Fatalf("didn't receive expected log")
	}
}

func TestAllEvents(t *testing.T) {
	t.Parallel()

	backend, addr, sc, authOpts, _ := testutil.Setup(t)
	qf, err := New(1337, backend, addr, eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)

	ctx, cls := context.WithCancel(context.Background())
	defer cls()
	chFeedClosed := make(chan struct{})
	ch := make(chan eventfeed.BlockEvents)
	go func() {
		err := qf.Start(ctx, 0, ch, []eventfeed.EventType{
			eventfeed.RunSQL,
			eventfeed.CreateTable,
			eventfeed.SetController,
			eventfeed.TransferTable,
		})
		require.NoError(t, err)
		close(chFeedClosed)
	}()

	ctrl := authOpts.From
	// Make four calls to different functions emitting different events
	_, err = sc.CreateTable(
		authOpts,
		ctrl,
		"CREATE TABLE foo (bar int)")
	require.NoError(t, err)

	_, err = sc.RunSQL(authOpts, ctrl, big.NewInt(1), "stmt-2")
	require.NoError(t, err)

	_, err = sc.SetController(
		authOpts,
		ctrl,
		big.NewInt(1),
		common.HexToAddress("0xB0Cf943Cf94E7B6A2657D15af41c5E06c2BFEA3E"),
	)
	require.NoError(t, err)

	_, err = sc.TransferFrom(
		authOpts,
		ctrl,
		common.HexToAddress("0xB0Cf943Cf94E7B6A2657D15af41c5E06c2BFEA3E"),
		big.NewInt(1),
	)
	require.NoError(t, err)
	backend.Commit()

	select {
	case bes := <-ch:
		require.Len(t, bes.TxnEvents, 4)
		require.NotEqual(t, emptyHash, bes.TxnEvents[0].TxnHash)
		require.NotEqual(t, emptyHash, bes.TxnEvents[1].TxnHash)
		require.IsType(t, &ethereum.ContractCreateTable{}, bes.TxnEvents[0].Events)
		require.IsType(t, &ethereum.ContractRunSQL{}, bes.TxnEvents[1].Events)
		require.IsType(t, &ethereum.ContractSetController{}, bes.TxnEvents[2].Events)
		require.IsType(t, &ethereum.ContractTransferTable{}, bes.TxnEvents[3].Events)
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

	qf, err := New(1337, conn, rinkebyContractAddr, eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)

	ctx, cls := context.WithCancel(context.Background())
	defer cls()
	chFeedClosed := make(chan struct{})
	ch := make(chan eventfeed.BlockEvents)
	go func() {
		contractDeploymentBlockNumber := 10140812 - 100
		err := qf.Start(ctx,
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
			ct := e.TxnEvents[0].Events.(*ethereum.ContractTransfer)
			fmt.Printf("blocknumber %d, %d events. (tokenId %d -> %s)\n", e.BlockNumber, len(e.TxnEvents), ct.TokenId, ct.To)
			num++
			if num > 40 {
				cls()
			}
		case <-chFeedClosed:
			return
		}
	}
}
