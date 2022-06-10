package client

import (
	"context"
	"encoding/hex"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/chains"
	"github.com/textileio/go-tableland/internal/router"
	"github.com/textileio/go-tableland/internal/router/middlewares"
	"github.com/textileio/go-tableland/internal/tableland"
	tblimpl "github.com/textileio/go-tableland/internal/tableland/impl"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	epimpl "github.com/textileio/go-tableland/pkg/eventprocessor/impl"
	"github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/user"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/tables/impl/testutil"
	txnpimpl "github.com/textileio/go-tableland/pkg/txn/impl"
	"github.com/textileio/go-tableland/pkg/util"
	"github.com/textileio/go-tableland/pkg/wallet"
	"github.com/textileio/go-tableland/tests"
)

func TestCreate(t *testing.T) {
	ctx, client, _, _, _, _ := setup(t)

	txn, err := client.Create(ctx, "CREATE TABLE foo_1337 (bar text)")
	require.NoError(t, err)
	require.NotEmpty(t, txn.Hash().Hex())
}

type aclHalfMock struct {
	sqlStore sqlstore.SystemStore
}

func (acl *aclHalfMock) CheckPrivileges(
	ctx context.Context,
	tx pgx.Tx,
	controller common.Address,
	id tableland.TableID,
	op tableland.Operation) (bool, error) {
	aclImpl := tblimpl.NewACL(acl.sqlStore, nil)
	return aclImpl.CheckPrivileges(ctx, tx, controller, id, op)
}

func (acl *aclHalfMock) IsOwner(ctx context.Context, controller common.Address, id tableland.TableID) (bool, error) {
	return true, nil
}

func setup(t *testing.T, opts ...parsing.Option) (
	context.Context,
	*Client,
	tableland.Tableland,
	*backends.SimulatedBackend,
	*ethereum.Contract,
	*bind.TransactOpts,
) {
	t.Helper()

	url := tests.PostgresURL(t)

	ctx := context.WithValue(context.Background(), middlewares.ContextKeyChainID, tableland.ChainID(1337))
	store, err := system.New(url, tableland.ChainID(1337))
	require.NoError(t, err)

	parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"}, opts...)
	require.NoError(t, err)

	txnp, err := txnpimpl.NewTxnProcessor(1337, url, 0, &aclHalfMock{store})
	require.NoError(t, err)

	backend, addr, sc, auth, sk := testutil.Setup(t)

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

	tbld := tblimpl.NewTablelandMesa(
		parser,
		userStore,
		chainStack,
	)

	router := router.ConfiguredRouter(
		"https://testnet.tableland.network",
		10,
		time.Second,
		parser,
		userStore,
		chainStack,
	)

	server := httptest.NewServer(router.Handler())

	bearer, err := util.AuthorizationSIWEValue(1337, wallet, time.Hour*24*365)
	require.NoError(t, err)

	rpcClient, err := rpc.DialContext(ctx, server.URL+"/rpc")
	require.NoError(t, err)

	rpcClient.SetHeader("Authorization", bearer)

	client := NewClient(Config{TblRPCClient: rpcClient, TblContractClient: registry})

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
		rpcClient.Close()
		server.Close()
	})

	return ctx, client, tbld, backend, sc, auth
}
