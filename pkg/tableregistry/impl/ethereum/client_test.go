package ethereum

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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestIsOwner(t *testing.T) {
	backend, key, fromAuth, contract, client := setup(t)
	_, toAuth := requireNewAuth(t)
	requireAuthGas(t, backend, toAuth)
	requireTxn(t, backend, key, fromAuth.From, toAuth.From, big.NewInt(1000000000000000000))
	tokenID := requireMint(t, backend, contract, toAuth, toAuth.From)

	owner, err := client.IsOwner(context.Background(), toAuth.From, tokenID)
	require.NoError(t, err)
	require.True(t, owner)

	owner, err = client.IsOwner(context.Background(), fromAuth.From, tokenID)
	require.NoError(t, err)
	require.False(t, owner)
}

func requireMint(
	t *testing.T,
	backend *backends.SimulatedBackend,
	contract *Contract,
	txOpts *bind.TransactOpts,
	to common.Address,
) *big.Int {
	txn, err := contract.SafeMint(txOpts, to)
	require.NoError(t, err)

	backend.Commit()

	receipt, err := backend.TransactionReceipt(context.Background(), txn.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)

	require.Len(t, receipt.Logs, 1)
	require.Len(t, receipt.Logs[0].Topics, 4)

	idBytes := receipt.Logs[0].Topics[3].Bytes()
	id := (&big.Int{}).SetBytes(idBytes)

	return id
}

func requireTxn(
	t *testing.T,
	backend *backends.SimulatedBackend,
	key *ecdsa.PrivateKey,
	from common.Address,
	to common.Address,
	amt *big.Int,
) {
	nonce, err := backend.PendingNonceAt(context.Background(), from)
	require.NoError(t, err)

	gasLimit := uint64(21000)
	gasPrice, err := backend.SuggestGasPrice(context.Background())
	require.NoError(t, err)

	var data []byte
	txnData := &types.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      gasLimit,
		To:       &to,
		Data:     data,
		Value:    amt,
	}
	tx := types.NewTx(txnData)
	signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, key)
	require.NoError(t, err)

	bal, err := backend.BalanceAt(context.Background(), from, nil)
	require.NoError(t, err)
	require.NotZero(t, bal)

	err = backend.SendTransaction(context.Background(), signedTx)
	require.NoError(t, err)

	backend.Commit()

	receipt, err := backend.TransactionReceipt(context.Background(), signedTx.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)
}

func requireAuthGas(t *testing.T, backend *backends.SimulatedBackend, auth *bind.TransactOpts) {
	gas, err := backend.SuggestGasPrice(context.Background())
	require.NoError(t, err)
	auth.GasPrice = gas
}

func requireNewAuth(t *testing.T) (*ecdsa.PrivateKey, *bind.TransactOpts) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	auth := bind.NewKeyedTransactor(key) //nolint
	return key, auth
}

func setup(t *testing.T) (*backends.SimulatedBackend, *ecdsa.PrivateKey, *bind.TransactOpts, *Contract, *Client) {
	key, auth := requireNewAuth(t)

	alloc := make(core.GenesisAlloc)
	alloc[auth.From] = core.GenesisAccount{Balance: big.NewInt(math.MaxInt64)}
	backend := backends.NewSimulatedBackend(alloc, math.MaxInt64)

	requireAuthGas(t, backend, auth)

	//Deploy contract
	address, _, contract, err := DeployContract(
		auth,
		backend,
	)

	// commit all pending transactions
	backend.Commit()

	require.NoError(t, err)

	if len(address.Bytes()) == 0 {
		t.Error("Expected a valid deployment address. Received empty address byte array instead")
	}

	client, err := NewClient(backend, address)
	require.NoError(t, err)

	return backend, key, auth, contract, client
}
