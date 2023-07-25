package fullstack

import (
	"encoding/hex"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/gateway"
	gatewayimpl "github.com/textileio/go-tableland/internal/gateway/impl"
	"github.com/textileio/go-tableland/internal/router"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/internal/tableland/impl"
	"github.com/textileio/go-tableland/pkg/database"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	epimpl "github.com/textileio/go-tableland/pkg/eventprocessor/impl"
	executor "github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor/impl"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"

	"github.com/textileio/go-tableland/pkg/sharedmemory"

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
	Wallet            *wallet.Wallet
	TblContractClient *ethereum.Client
	Server            *httptest.Server
}

// Deps holds possile dependencies that can optionally be provided to spin up the full stack.
type Deps struct {
	DBURI          string
	Parser         parsing.SQLValidator
	Database       *database.SQLiteDB
	ACL            tableland.ACL
	GatewayService gateway.Gateway
}

// CreateFullStack creates a running validator with the provided dependencies, or defaults otherwise.
func CreateFullStack(t *testing.T, deps Deps) FullStack {
	t.Helper()

	var err error

	parser := deps.Parser
	if parser == nil {
		parser, err = parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)
	}

	dbURI := deps.DBURI
	if dbURI == "" {
		dbURI = tests.Sqlite3URI(t)
	}

	db := deps.Database
	if db == nil {
		db, err = database.Open(dbURI)
		require.NoError(t, err)
	}

	backend, addr, contract, transactOpts, sk := testutil.Setup(t)

	wallet, err := wallet.NewWallet(hex.EncodeToString(crypto.FromECDSA(sk)))
	require.NoError(t, err)

	acl := deps.ACL
	if acl == nil {
		acl = impl.NewACL(db)
	}

	ex, err := executor.NewExecutor(1337, db, parser, 0, acl)
	require.NoError(t, err)

	sm := sharedmemory.NewSharedMemory()

	// Spin up dependencies needed for the EventProcessor.
	// i.e: Executor, Parser, and EventFeed (connected to the EVM chain)
	ef, err := efimpl.New(
		efimpl.NewEventFeedStore(db),
		ChainID,
		backend,
		addr,
		sm,
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
	})

	gatewayService := deps.GatewayService
	if gatewayService == nil {
		gatewayService, err = gateway.NewGateway(
			parser,
			gatewayimpl.NewGatewayStore(
				db, parsing.NewReadStatementResolver(sm),
			),
			"https://testnets.tableland.network",
			"https://tables.tableland.xyz",
			"https://tables.tableland.xyz",
		)
		require.NoError(t, err)
		gatewayService, err = gateway.NewInstrumentedGateway(gatewayService)
		require.NoError(t, err)
	}

	router, err := router.ConfiguredRouter(gatewayService, 10, time.Second, []tableland.ChainID{ChainID}, []string{})
	require.NoError(t, err)

	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)

	return FullStack{
		Backend:      backend,
		Address:      addr,
		Contract:     contract,
		TransactOpts: transactOpts,
		Wallet:       wallet,
		Server:       server,
	}
}
