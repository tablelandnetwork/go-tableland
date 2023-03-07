package tests

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

// SimulatedChain is simulated Ethereum backend with a contract deployed.
type SimulatedChain struct {
	ChainID int64
	Backend *backends.SimulatedBackend

	// deployer info
	DeployerPrivateKey   *ecdsa.PrivateKey
	DeployerTransactOpts *bind.TransactOpts
}

// Contract holds contract information and bindings.
type Contract struct {
	ContractAddr common.Address
	Contract     interface{} // it can be a Tableland Registry or Root Registry contract
}

// ContractDeployer represents a function that deploys a contract in the simulated backend.
type ContractDeployer func(
	*bind.TransactOpts,
	*backends.SimulatedBackend,
) (address common.Address, contract interface{}, err error)

// NewSimulatedChain creates a new simulated chain.
func NewSimulatedChain(t *testing.T) *SimulatedChain {
	c := &SimulatedChain{
		ChainID: 1337,
	}

	c.bootstrap(t)
	return c
}

func (c *SimulatedChain) bootstrap(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	transactOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(c.ChainID)) // nolint
	require.NoError(t, err)

	alloc := make(core.GenesisAlloc)
	alloc[transactOpts.From] = core.GenesisAccount{Balance: big.NewInt(math.MaxInt64)}
	backend := backends.NewSimulatedBackend(alloc, math.MaxInt64)
	gas, err := backend.SuggestGasPrice(context.Background())
	require.NoError(t, err)
	transactOpts.GasPrice = gas

	c.Backend = backend
	c.DeployerPrivateKey = key
	c.DeployerTransactOpts = transactOpts
}

// CreateAccountWithBalance creates a new account inside the simulated backend with balance and returns the private key.
func (c *SimulatedChain) CreateAccountWithBalance(t *testing.T) *ecdsa.PrivateKey {
	fromOpts, err := bind.NewKeyedTransactorWithChainID(c.DeployerPrivateKey, big.NewInt(c.ChainID))
	require.NoError(t, err)

	gasLimit := uint64(21000)
	gasPrice, err := c.Backend.SuggestGasPrice(context.Background())
	require.NoError(t, err)
	fromOpts.GasPrice = gasPrice

	nonce, err := c.Backend.PendingNonceAt(context.Background(), fromOpts.From)
	require.NoError(t, err)

	// generate random key
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	toOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(c.ChainID))
	require.NoError(t, err)

	var data []byte
	txnData := &types.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      gasLimit,
		To:       &toOpts.From,
		Data:     data,
		Value:    big.NewInt(1000000000000000000),
	}
	tx := types.NewTx(txnData)
	signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, c.DeployerPrivateKey)
	require.NoError(t, err)

	bal, err := c.Backend.BalanceAt(context.Background(), fromOpts.From, nil)
	require.NoError(t, err)
	require.NotZero(t, bal)

	err = c.Backend.SendTransaction(context.Background(), signedTx)
	require.NoError(t, err)

	c.Backend.Commit()

	receipt, err := c.Backend.TransactionReceipt(context.Background(), signedTx.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)

	return key
}

// DeployContract deploys a new contract to the chain.
func (c *SimulatedChain) DeployContract(t *testing.T, deploy ContractDeployer) (*Contract, error) {
	// Deploy contract
	address, contract, err := deploy(
		c.DeployerTransactOpts,
		c.Backend,
	)
	require.NoError(t, err)

	// commit all pending transactions
	c.Backend.Commit()
	require.NoError(t, err)

	if len(address.Bytes()) == 0 {
		t.Error("Expected a valid deployment address. Received empty address byte array instead")
	}

	return &Contract{
		ContractAddr: address,
		Contract:     contract,
	}, nil
}
