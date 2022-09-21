package tests

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"encoding/hex"
	"fmt"
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
	tblimpl "github.com/textileio/go-tableland/internal/tableland/impl"
	"github.com/textileio/go-tableland/pkg/client"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	epimpl "github.com/textileio/go-tableland/pkg/eventprocessor/impl"
	nonceimpl "github.com/textileio/go-tableland/pkg/nonce/impl"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	sqlstoresystemimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	sqlstoreuserimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl/user"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/tables/impl/testutil"
	"github.com/textileio/go-tableland/pkg/wallet"

	executor "github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor/impl"
)

type FullStack struct {
	Backand      *backends.SimulatedBackend
	Address      common.Address
	Contract     *ethereum.Contract
	TransactOpts *bind.TransactOpts
	Sk           *ecdsa.PrivateKey
	Wallet       *wallet.Wallet
	EthClient    *ethereum.Client
	Client       *client.Client
}

type config struct {
	systemStore sqlstore.SystemStore
	acl         tableland.ACL

	userStore     sqlstore.UserStore
	tableland     tableland.Tableland
	systemService system.SystemService
}

func CreateFullStack(t *testing.T) FullStack {
	t.Helper()

	ctx := context.Background()

	dbURI := Sqlite3URI()

	store, err := sqlstoresystemimpl.New(dbURI, tableland.ChainID(1337))
	require.NoError(t, err)

	parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
	require.NoError(t, err)

	ex, err := executor.NewExecutor(1337, dbURI, parser, 0, &aclHalfMock{store})
	require.NoError(t, err)

	backend, addr, contract, transactOpts, sk := testutil.Setup(t)

	wallet, err := wallet.NewWallet(hex.EncodeToString(crypto.FromECDSA(sk)))
	require.NoError(t, err)

	registry, err := ethereum.NewClient(
		backend,
		1337,
		addr,
		wallet,
		nonceimpl.NewSimpleTracker(wallet, backend),
	)
	require.NoError(t, err)

	userStore, err := sqlstoreuserimpl.New(dbURI)
	require.NoError(t, err)

	chainStacks := map[tableland.ChainID]chains.ChainStack{
		1337: {
			Store:                 store,
			Registry:              registry,
			AllowTransactionRelay: true,
		},
	}

	instrUserStore, err := sqlstoreimpl.NewInstrumentedUserStore(userStore)
	require.NoError(t, err)

	mesaService := impl.NewTablelandMesa(parser, instrUserStore, chainStacks)
	mesaService, err = impl.NewInstrumentedTablelandMesa(mesaService)
	require.NoError(t, err)

	stores := make(map[tableland.ChainID]sqlstore.SystemStore, len(chainStacks))
	for chainID, stack := range chainStacks {
		stores[chainID] = stack.Store
	}
	sysStore, err := systemimpl.NewSystemSQLStoreService(
		stores,
		"https://testnet.tableland.network",
		"https://render.tableland.xyz",
	)
	require.NoError(t, err)

	systemService, err := systemimpl.NewInstrumentedSystemSQLStoreService(sysStore)
	require.NoError(t, err)
	fmt.Println(systemService)

	// router := router.ConfiguredRouter(mesaService, systemService, 10, time.Second)
	router := router.ConfiguredRouter("", "", 0, time.Second, nil, nil, nil)

	server := httptest.NewServer(router.Handler())

	c := client.Chain{
		Endpoint:     server.URL,
		ID:           client.ChainID(1337),
		ContractAddr: addr,
	}

	client, err := client.NewClient(ctx, wallet, client.NewClientChain(c), client.NewClientContractBackend(backend))
	require.NoError(t, err)

	// Spin up dependencies needed for the EventProcessor.
	// i.e: Executor, Parser, and EventFeed (connected to the EVM chain)
	ef, err := efimpl.New(store, 1337, backend, addr, eventfeed.WithMinBlockDepth(0))
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

	return FullStack{
		Backand:      backend,
		Address:      addr,
		Contract:     contract,
		TransactOpts: transactOpts,
		Sk:           sk,
		Wallet:       wallet,
		EthClient:    registry,
		Client:       client,
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
	aclImpl := tblimpl.NewACL(acl.sqlStore, nil)
	return aclImpl.CheckPrivileges(ctx, tx, controller, id, op)
}

func (acl *aclHalfMock) IsOwner(_ context.Context, _ common.Address, _ tables.TableID) (bool, error) {
	return true, nil
}
