package testutil

import (
	"context"
	"crypto/ecdsa"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
)

// Setup spinup a simulated backend node connected to a test EVM chain running the Registry smart-contract.
func Setup(t *testing.T) (
	*backends.SimulatedBackend,
	common.Address,
	*ethereum.Contract,
	*bind.TransactOpts,
	*ecdsa.PrivateKey) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337)) //nolint
	require.NoError(t, err)

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

	return backend, address, contract, auth, key
}
