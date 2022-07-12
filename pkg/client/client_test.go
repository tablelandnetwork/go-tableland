package client

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/chains"
	"github.com/textileio/go-tableland/internal/router"
	"github.com/textileio/go-tableland/internal/tableland"
	tblimpl "github.com/textileio/go-tableland/internal/tableland/impl"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	epimpl "github.com/textileio/go-tableland/pkg/eventprocessor/impl"
	"github.com/textileio/go-tableland/pkg/nonce/impl"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/user"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/tables/impl/testutil"
	txnpimpl "github.com/textileio/go-tableland/pkg/txn/impl"
	"github.com/textileio/go-tableland/pkg/wallet"
	"github.com/textileio/go-tableland/tests"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	requireCreate(t, calls)
}

func TestList(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	requireCreate(t, calls)
	res := calls.list()
	require.Len(t, res, 1)
}

func TestWrite(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	_, table := requireCreate(t, calls)
	requireWrite(t, calls, table)
}

func TestRead(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	_, table := requireCreate(t, calls)
	hash := requireWrite(t, calls, table)
	requireReceipt(t, calls, hash, WaitFor(time.Second*10))
	res := calls.read(fmt.Sprintf("select * from %s", table))
	require.NotEmpty(t, res)
}

func TestHash(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	hash := calls.hash("create table foo_1337 (bar text)")
	require.NotEmpty(t, hash)
}

func TestSetController(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	tableID, _ := requireCreate(t, calls)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	controller := common.HexToAddress(crypto.PubkeyToAddress(key.PublicKey).Hex())
	hash := calls.setController(controller, tableID)
	require.NotEmpty(t, hash)
}

func requireCreate(t *testing.T, calls clientCalls) (TableID, string) {
	id, table := calls.create("(bar text)", WithPrefix("foo"), WithReceiptTimeout(time.Second*10))
	require.Equal(t, "foo_1337_1", table)
	return id, table
}

func requireWrite(t *testing.T, calls clientCalls, table string) string {
	hash := calls.write(fmt.Sprintf("insert into %s (bar) values('baz')", table))
	require.NotEmpty(t, hash)
	return hash
}

func requireReceipt(t *testing.T, calls clientCalls, hash string, opts ...ReceiptOption) *TxnReceipt {
	res, found := calls.receipt(hash, opts...)
	require.True(t, found)
	require.NotNil(t, res)
	return res
}

type aclHalfMock struct {
	sqlStore sqlstore.SystemStore
}

func (acl *aclHalfMock) CheckPrivileges(
	ctx context.Context,
	tx *sql.Tx,
	controller common.Address,
	id tableland.TableID,
	op tableland.Operation,
) (bool, error) {
	aclImpl := tblimpl.NewACL(acl.sqlStore, nil)
	return aclImpl.CheckPrivileges(ctx, tx, controller, id, op)
}

func (acl *aclHalfMock) IsOwner(ctx context.Context, controller common.Address, id tableland.TableID) (bool, error) {
	return true, nil
}

type clientCalls struct {
	list          func() []TableInfo
	create        func(schema string, opts ...CreateOption) (TableID, string)
	read          func(query string) string
	write         func(query string) string
	hash          func(statement string) string
	receipt       func(txnHash string, options ...ReceiptOption) (*TxnReceipt, bool)
	setController func(controller common.Address, tableID TableID) string
}

func setup(t *testing.T) clientCalls {
	t.Helper()

	ctx := context.Background()

	url := tests.Sqlite3URI()

	store, err := system.New(url, tableland.ChainID(1337))
	require.NoError(t, err)

	parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
	require.NoError(t, err)

	txnp, err := txnpimpl.NewTxnProcessor(1337, url, 0, &aclHalfMock{store})
	require.NoError(t, err)

	backend, addr, _, _, sk := testutil.Setup(t)

	wallet, err := wallet.NewWallet(hex.EncodeToString(crypto.FromECDSA(sk)))
	require.NoError(t, err)

	registry, err := ethereum.NewClient(
		backend,
		1337,
		addr,
		wallet,
		impl.NewSimpleTracker(wallet, backend),
	)
	require.NoError(t, err)

	userStore, err := user.New(url)
	require.NoError(t, err)

	chainStack := map[tableland.ChainID]chains.ChainStack{
		1337: {Store: store, Registry: registry},
	}

	router := router.ConfiguredRouter(
		"https://testnet.tableland.network",
		10,
		time.Second,
		parser,
		userStore,
		chainStack,
	)

	server := httptest.NewServer(router.Handler())

	client, err := NewClient(ctx, Config{
		TblAPIURL:    server.URL,
		EthBackend:   backend,
		ChainID:      1337,
		ContractAddr: addr,
		Wallet:       wallet,
	})
	require.NoError(t, err)

	// Spin up dependencies needed for the EventProcessor.
	// i.e: TxnProcessor, Parser, and EventFeed (connected to the EVM chain)
	ef, err := efimpl.New(1337, backend, addr, eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)

	// Create EventProcessor for our test.
	ep, err := epimpl.New(parser, txnp, ef, 1337)
	require.NoError(t, err)

	err = ep.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		ep.Stop()
		client.Close()
		server.Close()
	})

	return clientCalls{
		list: func() []TableInfo {
			res, err := client.List(ctx)
			require.NoError(t, err)
			return res
		},
		create: func(schema string, opts ...CreateOption) (TableID, string) {
			go func() {
				time.Sleep(time.Second * 1)
				backend.Commit()
			}()
			id, table, err := client.Create(ctx, schema, opts...)
			require.NoError(t, err)
			return id, table
		},
		read: func(query string) string {
			res, err := client.Read(ctx, query)
			require.NoError(t, err)
			return res
		},
		write: func(query string) string {
			hash, err := client.Write(ctx, query)
			require.NoError(t, err)
			backend.Commit()
			return hash
		},
		hash: func(statement string) string {
			hash, err := client.Hash(ctx, statement)
			require.NoError(t, err)
			return hash
		},
		receipt: func(txnHash string, options ...ReceiptOption) (*TxnReceipt, bool) {
			receipt, found, err := client.Receipt(ctx, txnHash, options...)
			require.NoError(t, err)
			return receipt, found
		},
		setController: func(controller common.Address, tableID TableID) string {
			hash, err := client.SetController(ctx, controller, tableID)
			require.NoError(t, err)
			backend.Commit()
			return hash
		},
	}
}
