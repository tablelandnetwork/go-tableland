package queryfeed

import (
	"context"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
)

func TestStart(t *testing.T) {
	t.Parallel()

	backend, addr, sc, authOpts := setup(t)

	controller := common.HexToAddress("0xB0Cf943Cf94E7B6A2657D15af41c5E06c2BFEA3D")
	qf, err := New(backend, addr)
	require.NoError(t, err)

	// Make one call before start listening.
	_, err = sc.RunSQL(authOpts, "tbl-1", controller, "stmt-1")
	require.NoError(t, err)
	backend.Commit()

	// Start listening to Logs for the contract from the next block.
	currBlockNumber := backend.Blockchain().CurrentHeader().Number.Int64()
	ch := make(chan BlockEvents)
	go func() {
		err := qf.Start(context.Background(), currBlockNumber+1, ch)
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
	_, err = sc.RunSQL(authOpts, "tbl-2", controller, "stmt-2")
	require.NoError(t, err)
	backend.Commit()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("didn't receive expected log")
	}

	// Try making two calls in a single block now, and assert we receive things correctly.
	_, err = sc.RunSQL(authOpts, "tbl-3", controller, "stmt-3")
	require.NoError(t, err)
	_, err = sc.RunSQL(authOpts, "tbl-4", controller, "stmt-4")
	require.NoError(t, err)
	backend.Commit()
	for i := 0; i < 2; i++ {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("didn't receive expected log")
		}
	}
}

func TestStartForTwoEventTypes(t *testing.T) {
	t.Parallel()

	backend, addr, sc, authOpts := setup(t)

	controller := common.HexToAddress("0xB0Cf943Cf94E7B6A2657D15af41c5E06c2BFEA3D")
	qf, err := New(backend, addr)
	require.NoError(t, err)

	ch := make(chan BlockEvents)
	go func() {
		err := qf.Start(context.Background(), 0, ch)
		require.NoError(t, err)
	}()

	// Make two calls to different functions emiting different events
	_, err = sc.RunSQL(authOpts, "tbl-2", controller, "stmt-2")
	require.NoError(t, err)
	_, err = sc.SafeMint(authOpts, common.HexToAddress("0xB0Cf943Cf94E7B6A2657D15af41c5E06c2BFEA3E"))
	require.NoError(t, err)
	backend.Commit()

	for i := 0; i < 2; i++ {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("didn't receive expected log")
		}
	}
}

// TOOD(jsign): TestStartCancelation(...)

func setup(t *testing.T) (*backends.SimulatedBackend, common.Address, *ethereum.Contract, *bind.TransactOpts) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	auth := bind.NewKeyedTransactor(key) //nolint

	alloc := make(core.GenesisAlloc)
	alloc[auth.From] = core.GenesisAccount{Balance: big.NewInt(math.MaxInt64)}
	backend := backends.NewSimulatedBackend(alloc, math.MaxInt64)
	gas, err := backend.SuggestGasPrice(context.Background())
	require.NoError(t, err)
	auth.GasPrice = gas

	//Deploy contract
	address, _, contract, err := ethereum.DeployContract(
		auth,
		backend,
	)

	// commit all pending transactions
	backend.Commit()

	require.NoError(t, err)

	if len(address.Bytes()) == 0 {
		t.Error("Expected a valid deployment address. Received empty address byte array instead")
	}
	return backend, address, contract, auth
}
