package impl

import (
	"context"
	"crypto/ecdsa"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/tableregistry"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
	"github.com/textileio/go-tableland/tests"
)

func TestCreateTable(t *testing.T) {
	ctx := context.Background()
	tableregistry, tableID, controller := ethSetup(t)

	url, err := tests.PostgresURL()
	require.NoError(t, err)

	sqlstore, err := sqlstoreimpl.New(ctx, url, true)
	require.NoError(t, err)

	mesa := NewTablelandMesa(sqlstore, tableregistry)

	_, err = sqlstore.GetTable(ctx, tableID)
	require.Error(t, err) // table does not exist on registry

	_, err = sqlstore.Read(ctx, "SELECT * FROM test;")
	require.Error(t, err) // table does not exist on user space

	_, err = mesa.CreateTable(ctx, tableland.Request{
		TableID:    tableID.String(),
		Controller: controller,
		Statement:  `CREATE TABLE "test" (a int);`,
	})
	require.NoError(t, err)

	table, err := sqlstore.GetTable(ctx, tableID)
	require.NoError(t, err) // table exists on registry
	require.Equal(t, table.UUID, tableID)

	data, err := sqlstore.Read(ctx, "SELECT * FROM test;")
	require.Empty(t, data.(map[string]interface{})["rows"])
	require.NoError(t, err) // table exists on user space
}
func TestCreateTableRollback(t *testing.T) {
	ctx := context.Background()
	tableregistry, tableID, _ := ethSetup(t) // ignore controller

	url, err := tests.PostgresURL()
	require.NoError(t, err)

	sqlstore, err := sqlstoreimpl.New(ctx, url, true)
	require.NoError(t, err)

	mesa := NewTablelandMesa(sqlstore, tableregistry)

	_, err = sqlstore.GetTable(ctx, tableID)
	require.Error(t, err) // table does not exist on registry

	_, err = sqlstore.Read(ctx, "SELECT * FROM test;")
	require.Error(t, err) // table does not exist on user space

	// use a controller bigger than 42 chars. this means that the CREATE TABLE will work but the INSERT INTO will not
	_, err = mesa.CreateTable(ctx, tableland.Request{
		TableID:    tableID.String(),
		Controller: "0xE3c13de334225F3a922589918149B751524d2Ef87", // bigger than 42, to force an error
		Statement:  `CREATE TABLE "test" (a int)`,
	})
	require.Error(t, err)

	_, err = sqlstore.GetTable(ctx, tableID)
	require.Error(t, err) // table does not exist on registry

	_, err = sqlstore.Read(ctx, "SELECT * FROM test;")
	require.Error(t, err) // table does not exist on registry
}

func TestCreateTableWithoutTransaction(t *testing.T) {
	ctx := context.Background()
	tableregistry, tableID, _ := ethSetup(t) // ignore controller

	url, err := tests.PostgresURL()
	require.NoError(t, err)

	sqlstore, err := sqlstoreimpl.New(ctx, url, false)
	require.NoError(t, err)

	mesa := NewTablelandMesa(sqlstore, tableregistry)

	_, err = sqlstore.GetTable(ctx, tableID)
	require.Error(t, err) // table does not exist on registry

	_, err = sqlstore.Read(ctx, "SELECT * FROM test;")
	require.Error(t, err) // table does not exist on user space

	// use a controller bigger than 42 chars. this means that the CREATE TABLE will work but the INSERT INTO will not
	_, err = mesa.CreateTable(ctx, tableland.Request{
		TableID:    tableID.String(),
		Controller: "0xE3c13de334225F3a922589918149B751524d2Ef87", // bigger than 42, to force an error
		Statement:  `CREATE TABLE "test" (a int)`,
	})
	require.Error(t, err)

	// the transaction was disabled. the rollback does not work and we have an incosistency state across tables
	_, err = sqlstore.GetTable(ctx, tableID)
	require.Error(t, err) // table does not exist on registry

	_, err = sqlstore.Read(ctx, "SELECT * FROM test;")
	require.NoError(t, err) // table does exist on registry
}

// TODO: create a utility methods for this inside the tests package.
func ethSetup(t *testing.T) (tableregistry.TableRegistry, uuid.UUID, string) {
	key, auth := requireNewAuth(t)

	alloc := make(core.GenesisAlloc)
	alloc[auth.From] = core.GenesisAccount{Balance: big.NewInt(math.MaxInt64)}
	backend := backends.NewSimulatedBackend(alloc, math.MaxInt64)

	requireAuthGas(t, backend, auth)

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

	registry, err := ethereum.NewClient(backend, address)
	require.NoError(t, err)

	_, toAuth := requireNewAuth(t)
	requireAuthGas(t, backend, toAuth)

	tableID := uuid.New()
	var n big.Int
	n.SetString(strings.Replace(tableID.String(), "-", "", 4), 16)

	requireTxn(t, backend, key, auth.From, toAuth.From, big.NewInt(1000000000000000000))
	requireMint(t, backend, contract, toAuth, toAuth.From, &n)

	return registry, tableID, toAuth.From.String()
}

func requireMint(
	t *testing.T,
	backend *backends.SimulatedBackend,
	contract *ethereum.Contract,
	txOpts *bind.TransactOpts,
	to common.Address,
	tableID *big.Int,
) *big.Int {
	tokenID := big.NewInt(0)

	txn, err := contract.Mint(txOpts, to, tokenID, tableID, nil)
	require.NoError(t, err)

	backend.Commit()

	receipt, err := backend.TransactionReceipt(context.Background(), txn.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)

	return tokenID
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
