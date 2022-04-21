package ethereum

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	nonceimpl "github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/wallet"
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

func TestRunSQL(t *testing.T) {
	backend, key, fromAuth, contract, _ := setup(t)
	_, toAuth := requireNewAuth(t)
	requireAuthGas(t, backend, toAuth)
	requireTxn(t, backend, key, fromAuth.From, toAuth.From, big.NewInt(1000000000000000000))

	addr := common.HexToAddress("0xB0Cf943Cf94E7B6A2657D15af41c5E06c2BFEA3D")
	requireRunSQL(t, backend, contract, fromAuth, tableland.TableID(*big.NewInt(1)), addr, "insert into XXX values (1,2,3)")
}

func requireRunSQL(
	t *testing.T,
	backend *backends.SimulatedBackend,
	contract *Contract,
	txOpts *bind.TransactOpts,
	tableID tableland.TableID,
	controller common.Address,
	statement string,
) {
	txn, err := contract.RunSQL(txOpts, tableID.ToBigInt(), controller, statement)
	require.NoError(t, err)

	backend.Commit()

	receipt, err := backend.TransactionReceipt(context.Background(), txn.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)

	require.Len(t, receipt.Logs, 1)
	require.Len(t, receipt.Logs[0].Topics, 1)

	contractAbi, err := abi.JSON(strings.NewReader(ContractMetaData.ABI))
	require.NoError(t, err)
	var event ContractRunSQL

	err = contractAbi.UnpackIntoInterface(&event, "RunSQL", receipt.Logs[0].Data)
	require.NoError(t, err)
	require.Equal(t, tableID.ToBigInt().Int64(), event.TableId.Int64())
	require.Equal(t, controller, event.Caller)
	require.Equal(t, statement, event.Statement)
}

func requireMint(
	t *testing.T,
	backend *backends.SimulatedBackend,
	contract *Contract,
	txOpts *bind.TransactOpts,
	to common.Address,
) *big.Int {
	txn, err := contract.CreateTable(txOpts, to, "CREATE TABLE foo (bar int)")
	require.NoError(t, err)

	backend.Commit()

	receipt, err := backend.TransactionReceipt(context.Background(), txn.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)

	require.Len(t, receipt.Logs, 2)
	require.Len(t, receipt.Logs[1].Topics, 1)

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
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337)) //nolint
	require.NoError(t, err)
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

	w, err := wallet.NewWallet(hex.EncodeToString(crypto.FromECDSA(key)))
	require.NoError(t, err)

	client, err := NewClient(backend, 4, address, w, nonceimpl.NewSimpleTracker(w, backend))
	require.NoError(t, err)

	return backend, key, auth, contract, client
}
