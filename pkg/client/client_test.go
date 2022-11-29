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
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/tables/impl/testutil"
	"github.com/textileio/go-tableland/pkg/wallet"
	"github.com/textileio/go-tableland/tests"

	executor "github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor/impl"
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

func TestRelayWrite(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	_, table := requireCreate(t, calls)
	requireWrite(t, calls, table, WriteRelay(true))
}

func TestDirectWrite(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	_, table := requireCreate(t, calls)
	requireWrite(t, calls, table, WriteRelay(false))
}

func TestRead(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	_, table := requireCreate(t, calls)
	hash := requireWrite(t, calls, table)
	requireReceipt(t, calls, hash, WaitFor(time.Second*10))

	type result struct {
		Bar string `json:"bar"`
	}

	res0 := []result{}
	calls.read(fmt.Sprintf("select * from %s", table), &res0)
	require.Len(t, res0, 1)
	require.Equal(t, "baz", res0[0].Bar)

	res1 := map[string]interface{}{}
	calls.read(fmt.Sprintf("select * from %s", table), &res1, ReadOutput(Table))
	require.Len(t, res1, 2)

	res2 := result{}
	calls.read(fmt.Sprintf("select * from %s", table), &res2, ReadUnwrap())
	require.Equal(t, "baz", res2.Bar)

	res3 := []string{}
	calls.read(fmt.Sprintf("select * from %s", table), &res3, ReadExtract())
	require.Len(t, res3, 1)
	require.Equal(t, "baz", res3[0])

	res4 := ""
	calls.read(fmt.Sprintf("select * from %s", table), &res4, ReadUnwrap(), ReadExtract())
	require.Equal(t, "baz", res4)
}

func TestHash(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	hash := calls.hash("create table foo_1337 (bar text)")
	require.NotEmpty(t, hash)
}

func TestValidate(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	id, table := requireCreate(t, calls)
	res := calls.validate(fmt.Sprintf("insert into %s (bar) values ('hi')", table))
	require.Equal(t, id, res)
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

func requireWrite(t *testing.T, calls clientCalls, table string, opts ...WriteOption) string {
	hash := calls.write(fmt.Sprintf("insert into %s (bar) values('baz')", table), opts...)
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
	id tables.TableID,
	op tableland.Operation,
) (bool, error) {
	aclImpl := tblimpl.NewACL(acl.sqlStore, nil)
	return aclImpl.CheckPrivileges(ctx, tx, controller, id, op)
}

func (acl *aclHalfMock) IsOwner(_ context.Context, _ common.Address, _ tables.TableID) (bool, error) {
	return true, nil
}

type clientCalls struct {
	list          func() []TableInfo
	create        func(schema string, opts ...CreateOption) (TableID, string)
	read          func(query string, target interface{}, opts ...ReadOption)
	write         func(query string, opts ...WriteOption) string
	hash          func(statement string) string
	validate      func(statement string) TableID
	receipt       func(txnHash string, options ...ReceiptOption) (*TxnReceipt, bool)
	setController func(controller common.Address, tableID TableID) string
}

func setup(t *testing.T) clientCalls {
	t.Helper()

	ctx := context.Background()

	dbURI := tests.Sqlite3URI(t)

	store, err := system.New(dbURI, tableland.ChainID(1337))
	require.NoError(t, err)

	parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", dbURI)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)

	ex, err := executor.NewExecutor(1337, db, parser, 0, &aclHalfMock{store})
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

	userStore, err := user.New(dbURI)
	require.NoError(t, err)

	chainStack := map[tableland.ChainID]chains.ChainStack{
		1337: {
			Store:                 store,
			Registry:              registry,
			AllowTransactionRelay: true,
		},
	}

	router, err := router.ConfiguredRouter(
		"https://testnet.tableland.network",
		"https://render.tableland.xyz",
		"",
		10,
		time.Second,
		parser,
		userStore,
		chainStack,
	)
	require.NoError(t, err)

	server := httptest.NewServer(router.Handler())

	c := Chain{
		Endpoint:     server.URL,
		ID:           ChainID(1337),
		ContractAddr: addr,
	}

	client, err := NewClient(ctx, wallet, NewClientChain(c), NewClientContractBackend(backend))
	require.NoError(t, err)

	// Spin up dependencies needed for the EventProcessor.
	// i.e: Executor, Parser, and EventFeed (connected to the EVM chain)
	ef, err := efimpl.New(
		store,
		1337,
		backend,
		addr,
		eventfeed.WithNewHeadPollFreq(time.Millisecond),
		eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)

	// Create EventProcessor for our test.
	ep, err := epimpl.New(parser, ex, ef, 1337)
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
		read: func(query string, target interface{}, opts ...ReadOption) {
			err := client.Read(ctx, query, target, opts...)
			require.NoError(t, err)
		},
		write: func(query string, opts ...WriteOption) string {
			hash, err := client.Write(ctx, query, opts...)
			require.NoError(t, err)
			backend.Commit()
			return hash
		},
		hash: func(statement string) string {
			hash, err := client.Hash(ctx, statement)
			require.NoError(t, err)
			return hash
		},
		validate: func(statement string) TableID {
			tableID, err := client.Validate(ctx, statement)
			require.NoError(t, err)
			return tableID
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
