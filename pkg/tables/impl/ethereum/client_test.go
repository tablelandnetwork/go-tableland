package ethereum

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	nonceimpl "github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum/test/controller"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum/test/erc721Enumerable"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum/test/erc721aQueryable"
	"github.com/textileio/go-tableland/pkg/wallet"
	"github.com/textileio/go-tableland/tests"
)

func TestCreateTable(t *testing.T) {
	t.Parallel()
	simulatedChain, client, _ := setup(t)
	backend := simulatedChain.Backend

	txn, err := client.CreateTable(
		context.Background(), simulatedChain.DeployerTransactOpts.From, "CREATE TABLE foo (bar int)",
	)
	require.NoError(t, err)
	backend.Commit()

	receipt, err := backend.TransactionReceipt(context.Background(), txn.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)

	// TODO: How many logs and topics should there be?
	require.Len(t, receipt.Logs, 2)
	require.Len(t, receipt.Logs[0].Topics, 4)
}

func TestIsOwner(t *testing.T) {
	t.Parallel()

	simulatedChain, client, contract := setup(t)
	backend, fromAuth := simulatedChain.Backend, simulatedChain.DeployerTransactOpts

	key := simulatedChain.CreateAccountWithBalance(t)
	toAuth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(simulatedChain.ChainID))
	require.NoError(t, err)

	tokenID := requireMint(t, backend, contract, toAuth, toAuth.From)

	owner, err := client.IsOwner(context.Background(), toAuth.From, tokenID)
	require.NoError(t, err)
	require.True(t, owner)

	owner, err = client.IsOwner(context.Background(), fromAuth.From, tokenID)
	require.NoError(t, err)
	require.False(t, owner)
}

func TestRunSQL(t *testing.T) {
	t.Parallel()

	simulatedChain, client, contract := setup(t)
	backend, txOpts := simulatedChain.Backend, simulatedChain.DeployerTransactOpts

	tokenID := requireMint(t, backend, contract, txOpts, txOpts.From)

	tableID, err := tables.NewTableID(tokenID.String())
	require.NoError(t, err)

	statement := "insert into foo_1 values (1,2,3)"

	txn, err := client.RunSQL(context.Background(), txOpts.From, tableID, statement)
	require.NoError(t, err)
	backend.Commit()

	receipt, err := backend.TransactionReceipt(context.Background(), txn.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)

	require.Len(t, receipt.Logs, 1)
	require.Len(t, receipt.Logs[0].Topics, 1)

	contractAbi, err := abi.JSON(strings.NewReader(ContractMetaData.ABI))
	require.NoError(t, err)

	event := &ContractRunSQL{}
	err = contractAbi.UnpackIntoInterface(event, "RunSQL", receipt.Logs[0].Data)

	require.NoError(t, err)
	require.Equal(t, tableID.ToBigInt().Int64(), event.TableId.Int64())
	require.True(t, event.Policy.AllowDelete)
	require.True(t, event.Policy.AllowInsert)
	require.True(t, event.Policy.AllowUpdate)
	require.Equal(t, "", event.Policy.WhereClause)
	require.Equal(t, []string{}, event.Policy.UpdatableColumns)
	require.Equal(t, "", event.Policy.WithCheck)
	require.Equal(t, statement, event.Statement)
	require.True(t, event.IsOwner)
}

func TestSetController(t *testing.T) {
	t.Parallel()

	simulatedChain, client, contract := setup(t)
	backend, contract, txOpts := simulatedChain.Backend,
		contract,
		simulatedChain.DeployerTransactOpts
	// You have to be the owner of the token to set the controller
	tokenID := requireMint(t, backend, contract, txOpts, txOpts.From)

	tableID, err := tables.NewTableID(tokenID.String())
	require.NoError(t, err)

	// Use the high-level Ethereum client to make the call.
	controller := common.HexToAddress("0x848D5C7d4bB9E4613B6bd2C421f88Db0D7F46C58")
	tx, err := client.SetController(context.Background(), txOpts.From, tableID, controller)
	require.NoError(t, err)
	backend.Commit()

	// With the tx hash check if the call did the right thing
	// by checking the event emitted.
	receipt, err := backend.TransactionReceipt(context.Background(), tx.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)

	require.Len(t, receipt.Logs, 1)
	require.Len(t, receipt.Logs[0].Topics, 1)

	contractAbi, err := abi.JSON(strings.NewReader(ContractMetaData.ABI))
	require.NoError(t, err)

	var event ContractSetController
	err = contractAbi.UnpackIntoInterface(&event, "SetController", receipt.Logs[0].Data)

	require.NoError(t, err)

	require.Equal(t, tokenID.Int64(), event.TableId.Int64())
	require.Equal(t, controller, event.Controller)
}

func TestRunSQLWithPolicy(t *testing.T) {
	t.Parallel()

	simulatedChain, client, contract := setup(t)
	backend, txOpts := simulatedChain.Backend, simulatedChain.DeployerTransactOpts

	// caller must be the sender
	callerAddress := simulatedChain.DeployerTransactOpts.From

	// Deploy controller contract
	controllerAddress, _, controllerContract, err := controller.DeployContract(
		simulatedChain.DeployerTransactOpts,
		backend,
	)
	require.NoError(t, err)
	backend.Commit()

	// Deploy erc721Enumerable contract
	erc721Address, _, erc721Contract, err := erc721Enumerable.DeployContract(
		txOpts,
		backend,
	)
	require.NoError(t, err)
	backend.Commit()

	// Deploy erc721aQueryable contract
	erc721aAddress, _, erc721aContract, err := erc721aQueryable.DeployContract(
		txOpts,
		backend,
	)
	require.NoError(t, err)
	backend.Commit()

	// Set contract addresses on controller
	_, err = controllerContract.SetFoos(txOpts, erc721Address)
	require.NoError(t, err)
	backend.Commit()
	_, err = controllerContract.SetBars(txOpts, erc721aAddress)
	require.NoError(t, err)
	backend.Commit()

	// You have to be the owner of the token to set the controller
	tokenID := requireMint(t, backend, contract, txOpts, callerAddress)
	tableID, err := tables.NewTableID(tokenID.String())
	require.NoError(t, err)

	_, err = client.SetController(context.Background(), callerAddress, tableID, controllerAddress)
	require.NoError(t, err)
	backend.Commit()

	// Controller requires caller to own a Foo and a Bar
	statement := "update testing_1 set baz = 1"
	_, err = client.RunSQL(context.Background(), callerAddress, tableID, statement)
	require.Error(t, err)

	// Mint two erc721 with ids 0 and 1
	_, err = erc721Contract.Mint(txOpts)
	require.NoError(t, err)
	backend.Commit()
	_, err = erc721Contract.Mint(txOpts)
	require.NoError(t, err)
	backend.Commit()

	// Mint one erc721a with id 0
	_, err = erc721aContract.Mint(txOpts)
	require.NoError(t, err)
	backend.Commit()

	// execute RunSQL with a controller previously set
	txn, err := client.RunSQL(context.Background(), callerAddress, tableID, statement)
	require.NoError(t, err)
	backend.Commit()

	receipt, err := backend.TransactionReceipt(context.Background(), txn.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)

	require.Len(t, receipt.Logs, 1)
	require.Len(t, receipt.Logs[0].Topics, 1)

	contractAbi, err := abi.JSON(strings.NewReader(ContractMetaData.ABI))
	require.NoError(t, err)

	event := &ContractRunSQL{}
	err = contractAbi.UnpackIntoInterface(event, "RunSQL", receipt.Logs[0].Data)

	require.NoError(t, err)
	require.Equal(t, tableID.ToBigInt().Int64(), event.TableId.Int64())
	require.False(t, event.Policy.AllowDelete)
	require.False(t, event.Policy.AllowInsert)
	require.True(t, event.Policy.AllowUpdate)
	require.Equal(t, "foo_id in (0,1) and bar_id in (0)", event.Policy.WhereClause)
	require.Equal(t, []string{"baz"}, event.Policy.UpdatableColumns)
	require.Equal(t, "baz > 0", event.Policy.WithCheck)
	require.Equal(t, statement, event.Statement)
}

func TestNonceTooLow(t *testing.T) {
	t.Parallel()

	// requireMint does a contract call to create a table.
	// In that process the nonce is increase but the tracker is not aware of it.
	// This simulates an out of sync nonce.
	// Try running this test with go test -v to see the retry happening.

	t.Run("run-sql", func(t *testing.T) {
		t.Parallel()

		simulatedChain, client, contract := setupWithLocalTracker(t)
		backend, txOpts := simulatedChain.Backend, simulatedChain.DeployerTransactOpts

		tokenID := requireMint(t, backend, contract, txOpts, txOpts.From)

		tableID, err := tables.NewTableID(tokenID.String())
		require.NoError(t, err)

		statement := "insert into foo_1 values (1,2,3)"

		_, err = client.RunSQL(context.Background(), txOpts.From, tableID, statement)
		require.NoError(t, err)
		backend.Commit()
	})

	t.Run("set-controller", func(t *testing.T) {
		t.Parallel()

		simulatedChain, client, contract := setupWithLocalTracker(t)
		backend, txOpts := simulatedChain.Backend, simulatedChain.DeployerTransactOpts

		tokenID := requireMint(t, backend, contract, txOpts, txOpts.From)

		tableID, err := tables.NewTableID(tokenID.String())
		require.NoError(t, err)

		// Use the high-level Ethereum client to make the call.
		controller := common.HexToAddress("0x848D5C7d4bB9E4613B6bd2C421f88Db0D7F46C58")
		_, err = client.SetController(context.Background(), txOpts.From, tableID, controller)
		require.NoError(t, err)
		backend.Commit()
	})
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

func setup(t *testing.T) (*tests.SimulatedChain, *Client, *Contract) {
	simulatedChain := tests.NewSimulatedChain(t)
	contract, err := simulatedChain.DeployContract(t, Deploy)
	require.NoError(t, err)

	w, err := wallet.NewWallet(hex.EncodeToString(crypto.FromECDSA(simulatedChain.DeployerPrivateKey)))
	require.NoError(t, err)

	client, err := NewClient(
		simulatedChain.Backend,
		tableland.ChainID(simulatedChain.ChainID),
		contract.ContractAddr,
		w,
		nonceimpl.NewSimpleTracker(w, simulatedChain.Backend),
	)
	require.NoError(t, err)

	return simulatedChain, client, contract.Contract.(*Contract)
}

func setupWithLocalTracker(t *testing.T) (*tests.SimulatedChain, *Client, *Contract) {
	simulatedChain := tests.NewSimulatedChain(t)
	contract, err := simulatedChain.DeployContract(t, Deploy)
	require.NoError(t, err)

	w, err := wallet.NewWallet(hex.EncodeToString(crypto.FromECDSA(simulatedChain.DeployerPrivateKey)))
	require.NoError(t, err)

	url := tests.Sqlite3URI(t)

	systemStore, err := system.New(url, tableland.ChainID(1337))
	require.NoError(t, err)

	tracker, err := nonceimpl.NewLocalTracker(
		context.Background(),
		w,
		nonceimpl.NewNonceStore(systemStore),
		tableland.ChainID(1337),
		simulatedChain.Backend,
		5*time.Second,
		0,
		3*time.Microsecond,
	)
	require.NoError(t, err)

	client, err := NewClient(simulatedChain.Backend, 1337, contract.ContractAddr, w, tracker)
	require.NoError(t, err)

	return simulatedChain, client, contract.Contract.(*Contract)
}
