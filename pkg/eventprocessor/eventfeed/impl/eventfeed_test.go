package impl

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/testutil"
)

func TestStart(t *testing.T) {
	t.Parallel()

	backend, addr, sc, authOpts := testutil.Setup(t)
	qf, err := New(backend, addr, eventfeed.WithMinBlockChainDepth(0))
	require.NoError(t, err)

	ctrl := common.HexToAddress("0xB0Cf943Cf94E7B6A2657D15af41c5E06c2BFEA3D")
	// Make one call before start listening.
	_, err = sc.RunSQL(authOpts, "tbl-1", ctrl, "stmt-1")
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
	_, err = sc.RunSQL(authOpts, "tbl-2", ctrl, "stmt-2")
	require.NoError(t, err)
	backend.Commit()
	select {
	case bes := <-ch:
		require.Len(t, bes.Events, 1)
		require.IsType(t, &ethereum.ContractRunSQL{}, bes.Events[0])
	case <-time.After(time.Second):
		t.Fatalf("didn't receive expected log")
	}

	// Try making two calls in a single block now, and assert we receive things correctly.
	_, err = sc.RunSQL(authOpts, "tbl-3", ctrl, "stmt-3")
	require.NoError(t, err)
	_, err = sc.RunSQL(authOpts, "tbl-4", ctrl, "stmt-4")
	require.NoError(t, err)
	backend.Commit()
	select {
	case bes := <-ch:
		require.Len(t, bes.Events, 2)
		require.IsType(t, &ethereum.ContractRunSQL{}, bes.Events[0])
		require.IsType(t, &ethereum.ContractRunSQL{}, bes.Events[1])
	case <-time.After(time.Second):
		t.Fatalf("didn't receive expected log")
	}
}

func TestStartForTwoEventTypes(t *testing.T) {
	t.Parallel()

	backend, addr, sc, authOpts := testutil.Setup(t)
	qf, err := New(backend, addr, eventfeed.WithMinBlockChainDepth(0))
	require.NoError(t, err)

	ch := make(chan eventfeed.BlockEvents)
	go func() {
		err := qf.Start(context.Background(), 0, ch, []eventfeed.EventType{eventfeed.RunSQL, eventfeed.Transfer})
		require.NoError(t, err)
	}()

	ctrl := common.HexToAddress("0xB0Cf943Cf94E7B6A2657D15af41c5E06c2BFEA3D")
	// Make two calls to different functions emiting different events
	_, err = sc.RunSQL(authOpts, "tbl-2", ctrl, "stmt-2")
	require.NoError(t, err)
	_, err = sc.SafeMint(authOpts, common.HexToAddress("0xB0Cf943Cf94E7B6A2657D15af41c5E06c2BFEA3E"))
	require.NoError(t, err)
	backend.Commit()

	select {
	case bes := <-ch:
		require.Len(t, bes.Events, 2)
		require.IsType(t, &ethereum.ContractRunSQL{}, bes.Events[0])
		require.IsType(t, &ethereum.ContractTransfer{}, bes.Events[1])
	case <-time.After(time.Second):
		t.Fatalf("didn't receive expected log")
	}
}

// TOOD(jsign): TestStartCancelation(...)
