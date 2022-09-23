package testutils

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"encoding/hex"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/chains"
	"github.com/textileio/go-tableland/internal/router"
	"github.com/textileio/go-tableland/internal/system"
	systemimpl "github.com/textileio/go-tableland/internal/system/impl"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/internal/tableland/impl"
	"github.com/textileio/go-tableland/pkg/client"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	epimpl "github.com/textileio/go-tableland/pkg/eventprocessor/impl"
	executor "github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor/impl"
	nonceimpl "github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	sqlstoreimplsystem "github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	sqlstoreimpluser "github.com/textileio/go-tableland/pkg/sqlstore/impl/user"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/tables/impl/testutil"
	"github.com/textileio/go-tableland/pkg/wallet"
	"github.com/textileio/go-tableland/tests"
)

// ChainID is the test chain id.
var ChainID = tableland.ChainID(1337)

// FullStack holds all potentially useful components of the Tableland test stack.
type FullStack struct {
	Backend           *backends.SimulatedBackend
	Address           common.Address
	Contract          *ethereum.Contract
	TransactOpts      *bind.TransactOpts
	Sk                *ecdsa.PrivateKey
	Wallet            *wallet.Wallet
	TblContractClient *ethereum.Client
	Client            *client.Client
}

// Deps holds possile dependencies that can optionally be provided to spin up the full stack.
type Deps struct {
	DBURI         string
	Parser        parsing.SQLValidator
	SystemStore   sqlstore.SystemStore
	UserStore     sqlstore.UserStore
	ACL           tableland.ACL
	Tableland     tableland.Tableland
	SystemService system.SystemService
}

// CreateFullStack creates the full stack, returning possibly useful things.
func CreateFullStack(t *testing.T, deps Deps) FullStack {
	t.Helper()

	ctx := context.Background()
	var err error

	parser := deps.Parser
	if parser == nil {
		parser, err = parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)
	}

	dbURI := deps.DBURI
	if dbURI == "" {
		dbURI = tests.Sqlite3URI()
	}

	systemStore := deps.SystemStore
	if systemStore == nil {
		systemStore, err = sqlstoreimplsystem.New(dbURI, ChainID)
		require.NoError(t, err)
	}

	userStore := deps.UserStore
	if userStore == nil {
		userStore, err = sqlstoreimpluser.New(dbURI)
		require.NoError(t, err)
		userStore, err = sqlstoreimpl.NewInstrumentedUserStore(userStore)
		require.NoError(t, err)
	}

	backend, addr, contract, transactOpts, sk := testutil.Setup(t)

	wallet, err := wallet.NewWallet(hex.EncodeToString(crypto.FromECDSA(sk)))
	require.NoError(t, err)

	registry, err := ethereum.NewClient(
		backend,
		ChainID,
		addr,
		wallet,
		nonceimpl.NewSimpleTracker(wallet, backend),
	)
	require.NoError(t, err)

	chainStacks := map[tableland.ChainID]chains.ChainStack{
		ChainID: {
			Store:                 systemStore,
			Registry:              registry,
			AllowTransactionRelay: true,
		},
	}

	tbl := deps.Tableland
	if tbl == nil {
		tbl = impl.NewTablelandMesa(parser, userStore, chainStacks)
		tbl, err = impl.NewInstrumentedTablelandMesa(tbl)
		require.NoError(t, err)
	}

	stores := make(map[tableland.ChainID]sqlstore.SystemStore, len(chainStacks))
	for chainID, stack := range chainStacks {
		stores[chainID] = stack.Store
	}

	systemService := deps.SystemService
	if systemService == nil {
		systemService, err = systemimpl.NewSystemSQLStoreService(
			stores,
			"https://testnet.tableland.network",
			"https://render.tableland.xyz",
		)
		require.NoError(t, err)
		systemService, err = systemimpl.NewInstrumentedSystemSQLStoreService(systemService)
		require.NoError(t, err)
	}

	acl := deps.ACL
	if acl == nil {
		acl = &aclHalfMock{systemStore}
	}

	ex, err := executor.NewExecutor(ChainID, dbURI, parser, 0, acl)
	require.NoError(t, err)

	router := router.ConfiguredRouter(tbl, systemService, 10, time.Second)

	server := httptest.NewServer(router.Handler())

	c := client.Chain{
		Endpoint:     server.URL,
		ID:           client.ChainID(ChainID),
		ContractAddr: addr,
	}

	client, err := client.NewClient(ctx, wallet, client.NewClientChain(c), client.NewClientContractBackend(backend))
	require.NoError(t, err)

	// Spin up dependencies needed for the EventProcessor.
	// i.e: Executor, Parser, and EventFeed (connected to the EVM chain)
	ef, err := efimpl.New(systemStore, ChainID, backend, addr, eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)

	// Create EventProcessor for our test.
	ep, err := epimpl.New(parser, ex, ef, ChainID)
	require.NoError(t, err)

	err = ep.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		ep.Stop()
		client.Close()
		server.Close()
	})

	return FullStack{
		Backend:           backend,
		Address:           addr,
		Contract:          contract,
		TransactOpts:      transactOpts,
		Sk:                sk,
		Wallet:            wallet,
		TblContractClient: registry,
		Client:            client,
	}
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
	aclImpl := impl.NewACL(acl.sqlStore, nil)
	return aclImpl.CheckPrivileges(ctx, tx, controller, id, op)
}

func (acl *aclHalfMock) IsOwner(_ context.Context, _ common.Address, _ tables.TableID) (bool, error) {
	return true, nil
}
