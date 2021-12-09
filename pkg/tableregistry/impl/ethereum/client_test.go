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
	backend, key, txnOpts, contract, client := setup(t)
	toTxnOpts := requireNewTxnOpts(t, backend)
	requireTxn(t, backend, key, txnOpts.From, toTxnOpts.From, big.NewInt(1000000000000000000))
	tokenID := requireMint(t, backend, contract, toTxnOpts, toTxnOpts.From)

	owner, err := client.IsOwner(context.Background(), toTxnOpts.From, tokenID)
	require.NoError(t, err)
	require.True(t, owner)

	owner, err = client.IsOwner(context.Background(), txnOpts.From, tokenID)
	require.NoError(t, err)
	require.False(t, owner)
}

func requireMint(t *testing.T, backend *backends.SimulatedBackend, contract *Contract, txOpts *bind.TransactOpts, to common.Address) *big.Int {
	tokenID := big.NewInt(0)
	txn, err := contract.Mint(txOpts, to, tokenID, big.NewInt(1), nil)
	require.NoError(t, err)

	backend.Commit()

	receipt, err := backend.TransactionReceipt(context.Background(), txn.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)

	return tokenID
}

func requireTxn(t *testing.T, backend *backends.SimulatedBackend, key *ecdsa.PrivateKey, from, to common.Address, amt *big.Int) {
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

func requireNewTxnOpts(t *testing.T, backend *backends.SimulatedBackend) *bind.TransactOpts {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	auth := bind.NewKeyedTransactor(key)

	gas, err := backend.SuggestGasPrice(context.Background())
	require.NoError(t, err)
	auth.GasPrice = gas

	return auth
}

func setup(t *testing.T) (*backends.SimulatedBackend, *ecdsa.PrivateKey, *bind.TransactOpts, *Contract, *Client) {
	//Setup simulated block chain
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	auth := bind.NewKeyedTransactor(key)

	alloc := make(core.GenesisAlloc)
	alloc[auth.From] = core.GenesisAccount{Balance: big.NewInt(math.MaxInt64)}
	blockchain := backends.NewSimulatedBackend(alloc, math.MaxInt64)

	gas, err := blockchain.SuggestGasPrice(context.Background())
	require.NoError(t, err)
	auth.GasPrice = gas

	//Deploy contract
	address, _, contract, err := DeployContract(
		auth,
		blockchain,
	)
	// commit all pending transactions
	blockchain.Commit()

	require.NoError(t, err)

	if len(address.Bytes()) == 0 {
		t.Error("Expected a valid deployment address. Received empty address byte array instead")
	}

	client, err := NewClient(blockchain, address)
	require.NoError(t, err)

	return blockchain, key, auth, contract, client
}
