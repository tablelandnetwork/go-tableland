package fullstack

import (
	"context"
	"database/sql"
	"encoding/hex"
	"net/http/httptest"
	"os"
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
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	epimpl "github.com/textileio/go-tableland/pkg/eventprocessor/impl"
	executor "github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor/impl"
	merklepublisher "github.com/textileio/go-tableland/pkg/merkletree/publisher"
	merklepublisherimpl "github.com/textileio/go-tableland/pkg/merkletree/publisher/impl"
	nonceimpl "github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	rsresolver "github.com/textileio/go-tableland/pkg/readstatementresolver"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	sqlstoreimplsystem "github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/user"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
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
	DBURI         string
	Parser        parsing.SQLValidator
	SystemStore   sqlstore.SystemStore
	UserStore     sqlstore.UserStore
	ACL           tableland.ACL
	Tableland     tableland.Tableland
	SystemService system.SystemService
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

	systemStore := deps.SystemStore
	if systemStore == nil {
		systemStore, err = sqlstoreimplsystem.New(dbURI, ChainID)
		require.NoError(t, err)
	}

	// Spin up the EVM chain with the contract.
	simulatedChain := tests.NewSimulatedChain(t)
	contract, err := simulatedChain.DeployContract(t, ethereum.Deploy)
	require.NoError(t, err)

	wallet, err := wallet.NewWallet(hex.EncodeToString(crypto.FromECDSA(simulatedChain.DeployerPrivateKey)))
	require.NoError(t, err)

	registry, err := ethereum.NewClient(
		simulatedChain.Backend,
		ChainID,
		contract.ContractAddr,
		wallet,
		nonceimpl.NewSimpleTracker(wallet, simulatedChain.Backend),
	)
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", dbURI)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)

	acl := deps.ACL
	if acl == nil {
		acl = &aclHalfMock{systemStore}
	}

	ex, err := executor.NewExecutor(1337, db, parser, 0, acl)
	require.NoError(t, err)
	// Spin up dependencies needed for the EventProcessor.
	// i.e: Executor, Parser, and EventFeed (connected to the EVM chain)
	ef, err := efimpl.New(
		systemStore,
		ChainID,
		simulatedChain.Backend,
		contract.ContractAddr,
		eventfeed.WithNewHeadPollFreq(time.Millisecond),
		eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)

	// Create EventProcessor for our test.
	ep, err := epimpl.New(parser, ex, ef, 1337, eventprocessor.WithHashCalcStep(1))
	require.NoError(t, err)

	err = ep.Start()
	require.NoError(t, err)

	chainStacks := map[tableland.ChainID]chains.ChainStack{
		1337: {
			Store:                 systemStore,
			Registry:              registry,
			AllowTransactionRelay: true,
			EventProcessor:        ep,
		},
	}

	tbl := deps.Tableland
	if tbl == nil {
		userStore := deps.UserStore
		if userStore == nil {
			userStore, err = user.New(
				dbURI,
				rsresolver.New(map[tableland.ChainID]eventprocessor.EventProcessor{1337: ep}),
			)
			require.NoError(t, err)
		}
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
			"https://testnets.tableland.network",
			"https://render.tableland.xyz",
			"https://render.tableland.xyz/anim",
		)
		require.NoError(t, err)
		systemService, err = systemimpl.NewInstrumentedSystemSQLStoreService(systemService)
		require.NoError(t, err)
	}

	treeStore, err := merklepublisherimpl.NewMerkleTreeStore(tempfile(t))
	require.NoError(t, err)

	merkleRootContract, err := simulatedChain.DeployContract(t,
		func(auth *bind.TransactOpts, sb *backends.SimulatedBackend) (common.Address, interface{}, error) {
			addr, _, contract, err := merklepublisherimpl.DeployContract(auth, sb)
			return addr, contract, err
		})
	require.NoError(t, err)

	rootRegistry, err := merklepublisherimpl.NewMerkleRootRegistryEthereum(
		simulatedChain.Backend,
		merkleRootContract.ContractAddr,
		wallet,
		nonceimpl.NewSimpleTracker(wallet, simulatedChain.Backend),
	)
	require.NoError(t, err)

	merkleRootPublisher := merklepublisher.NewMerkleRootPublisher(
		merklepublisherimpl.NewLeavesStore(systemStore),
		treeStore,
		rootRegistry,
		time.Second,
	)
	merkleRootPublisher.Start()

	router, err := router.ConfiguredRouter(tbl, systemService, treeStore, 10, time.Second, []tableland.ChainID{ChainID})
	require.NoError(t, err)

	server := httptest.NewServer(router.Handler())

	t.Cleanup(func() {
		server.Close()
		ep.Stop()
		merkleRootPublisher.Close()
	})

	return FullStack{
		Backend:           simulatedChain.Backend,
		Address:           contract.ContractAddr,
		Contract:          contract.Contract.(*ethereum.Contract),
		TransactOpts:      simulatedChain.DeployerTransactOpts,
		Wallet:            wallet,
		TblContractClient: registry,
		Server:            server,
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

// tempfile returns a temporary file path.
func tempfile(t *testing.T) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "bolt_*.db")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	return f.Name()
}
