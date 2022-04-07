package queryfeed

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"strconv"
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
	backend, addr, sc, authOpts := setup(t)

	qf := New(backend, addr)

	ch := make(chan MutStatement)
	go func() {
		err := qf.Start(context.Background(), 0, ch)
		require.NoError(t, err)
	}()
	go func() {
		controller := common.HexToAddress("0xB0Cf943Cf94E7B6A2657D15af41c5E06c2BFEA3D")
		var i int
		for {
			table := strconv.Itoa(i)
			stmt := fmt.Sprintf("insert into %d", i)
			_, err := sc.RunSQL(authOpts, table, controller, stmt)
			require.NoError(t, err)
			fmt.Println("WIN")
			i++
			backend.Commit()
			<-time.After(time.Second)
		}
	}()
	for mq := range ch {
		fmt.Printf("New event: %#v", mq)
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
