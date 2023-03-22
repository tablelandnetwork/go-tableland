package impl

import (
	"context"
	"database/sql"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	executor "github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor/impl"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	rsresolver "github.com/textileio/go-tableland/pkg/readstatementresolver"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/testutil"
	"github.com/textileio/go-tableland/tests"
)

const chainID = 1337

func TestRunSQLBlockProcessing(t *testing.T) {
	t.Parallel()

	tableID, err := tables.NewTableID("1")
	require.NoError(t, err)

	expWrongTypeErr := "db query execution failed (code: SQLITE_table test_1337_1 has 1 columns but 2 values were supplied, msg: table test_1337_1 has 1 columns but 2 values were supplied)" //nolint
	cond := func(dr dbReader, exp []int64) func() bool {
		return func() bool {
			got := dr("select * from test_1337_1")
			if len(exp) != len(got) {
				return false
			}
			for i := range exp {
				if exp[i] != got[i] {
					return false
				}
			}
			return true
		}
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		contractCalls, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1337_1 values (1001)"}
		txnHashes := contractCalls.runSQL(queries)

		expReceipt := eventprocessor.Receipt{
			ChainID: chainID,
			TxnHash: txnHashes[0].String(),
			Error:   nil,
			TableID: &tableID,
		}
		require.Eventually(t, checkReceipts(t, expReceipt), time.Second*5, time.Millisecond*100)

		expectedRows := []int64{1001}
		require.Eventually(t, cond(dbReader, expectedRows), time.Second, time.Millisecond*100)
	})
	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		contractCalls, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1337_1 values (1,2)"}
		txnHashes := contractCalls.runSQL(queries)

		expReceipt := eventprocessor.Receipt{
			ChainID: chainID,
			TxnHash: txnHashes[0].String(),
			Error:   &expWrongTypeErr,
			TableID: nil,
		}
		require.Eventually(t, checkReceipts(t, expReceipt), time.Second*5, time.Millisecond*100)

		notExpectedRows := []int64{1001}
		require.Never(t, cond(dbReader, notExpectedRows), time.Second, time.Millisecond*100)
	})
	t.Run("success-success", func(t *testing.T) {
		t.Parallel()

		contractCalls, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1337_1 values (1001)", "insert into test_1337_1 values (1002)"}
		txnHashes := contractCalls.runSQL(queries)

		expReceipts := make([]eventprocessor.Receipt, len(txnHashes))
		for i, th := range txnHashes {
			expReceipts[i] = eventprocessor.Receipt{
				ChainID:      chainID,
				TxnHash:      th.String(),
				IndexInBlock: int64(i),
				Error:        nil,
				TableID:      &tableID,
			}
		}
		require.Eventually(t, checkReceipts(t, expReceipts...), time.Second*5, time.Millisecond*100)

		expectedRows := []int64{1001, 1002}
		require.Eventually(t, cond(dbReader, expectedRows), time.Second, time.Millisecond*100)
	})
	t.Run("failure-success", func(t *testing.T) {
		t.Parallel()

		contractCalls, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1337_1 values (1,2)", "insert into test_1337_1 values (1002)"}
		txnHashes := contractCalls.runSQL(queries)

		expReceipts := make([]eventprocessor.Receipt, len(txnHashes))
		expReceipts[0] = eventprocessor.Receipt{
			ChainID:      chainID,
			TxnHash:      txnHashes[0].String(),
			IndexInBlock: 0,
			Error:        &expWrongTypeErr,
			TableID:      nil,
		}
		expReceipts[1] = eventprocessor.Receipt{
			ChainID:      chainID,
			TxnHash:      txnHashes[1].String(),
			IndexInBlock: 1,
			Error:        nil,
			TableID:      &tableID,
		}
		require.Eventually(t, checkReceipts(t, expReceipts...), time.Second*5, time.Millisecond*100)

		expectedRows := []int64{1002}
		require.Eventually(t, cond(dbReader, expectedRows), time.Second, time.Millisecond*100)
	})
	t.Run("success-failure", func(t *testing.T) {
		t.Parallel()
		contractCalls, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1337_1 values (1001)", "insert into test_1337_1 values (1,2)"}
		txnHashes := contractCalls.runSQL(queries)

		expReceipts := make([]eventprocessor.Receipt, len(txnHashes))
		expReceipts[0] = eventprocessor.Receipt{
			ChainID:      chainID,
			TxnHash:      txnHashes[0].String(),
			IndexInBlock: 0,
			Error:        nil,
			TableID:      &tableID,
		}
		expReceipts[1] = eventprocessor.Receipt{
			ChainID:      chainID,
			TxnHash:      txnHashes[1].String(),
			IndexInBlock: 1,
			Error:        &expWrongTypeErr,
			TableID:      nil,
		}
		require.Eventually(t, checkReceipts(t, expReceipts...), time.Second*5, time.Millisecond*100)

		expectedRows := []int64{1001}
		require.Eventually(t, cond(dbReader, expectedRows), time.Second, time.Millisecond*100)
	})
}

func TestCreateTableBlockProcessing(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		contractCalls, checkReceipts, _ := setup(t)
		for i := 0; i < 2; i++ {
			txnHash := contractCalls.createTable("CREATE TABLE Foo_1337 (bar int)")

			tableID, err := tables.NewTableID(strconv.Itoa(i + 2))
			require.NoError(t, err)
			expReceipt := eventprocessor.Receipt{
				ChainID: chainID,
				TxnHash: txnHash.String(),
				Error:   nil,
				TableID: &tableID,
			}
			require.Eventually(t, checkReceipts(t, expReceipt), time.Second*5, time.Millisecond*100)
		}
	})

	expWrongTypeErr := "query validation: unable to parse the query: syntax error at position 7 near 'CREATEZ'"
	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		contractCalls, checkReceipts, _ := setup(t)
		txnHash := contractCalls.createTable("CREATEZ TABLE Foo_1337 (bar int)")

		expReceipt := eventprocessor.Receipt{
			ChainID: chainID,
			TxnHash: txnHash.String(),
			Error:   &expWrongTypeErr,
			TableID: nil,
		}
		require.Eventually(t, checkReceipts(t, expReceipt), time.Second*5, time.Millisecond*100)
	})
}

func TestQueryWithWrongTableTarget(t *testing.T) {
	t.Parallel()

	contractCalls, checkReceipts, _ := setup(t)

	// Note that we make a query for table 9999 instead of 1 which was
	// provided in the SC runSQL call.
	queries := []string{"insert into test_1337_9999 values (1001)"}
	txnHashes := contractCalls.runSQL(queries)

	expErr := "query targets table id 9999 and not 1"
	expReceipt := eventprocessor.Receipt{
		ChainID: chainID,
		TxnHash: txnHashes[0].String(),
		Error:   &expErr,
		TableID: nil,
	}
	require.Eventually(t, checkReceipts(t, expReceipt), time.Second*5, time.Millisecond*100)
}

func TestSetController(t *testing.T) {
	t.Parallel()

	contractCalls, checkReceipts, _ := setup(t)

	t.Run("set-controller", func(t *testing.T) {
		controller := common.HexToAddress("0x39b1b9B439312Dd9E1aE137ce9866e873eA4d211")
		txnHash := contractCalls.setController(controller)

		tid := tables.TableID(*big.NewInt(1))
		expReceipt := eventprocessor.Receipt{
			ChainID: chainID,
			TxnHash: txnHash.Hex(),
			Error:   nil,
			TableID: &tid,
		}
		require.Eventually(t, checkReceipts(t, expReceipt), time.Second*5, time.Millisecond*100)
	})

	t.Run("unset-controller", func(t *testing.T) {
		controller := common.HexToAddress("0x0")
		txnHash := contractCalls.setController(controller)

		tid := tables.TableID(*big.NewInt(1))
		expReceipt := eventprocessor.Receipt{
			ChainID: chainID,
			TxnHash: txnHash.Hex(),
			Error:   nil,
			TableID: &tid,
		}
		require.Eventually(t, checkReceipts(t, expReceipt), time.Second*5, time.Millisecond*100)
	})
}

func TestTransfer(t *testing.T) {
	t.Parallel()

	contractCalls, checkReceipts, _ := setup(t)

	txnHash := contractCalls.createTable("CREATE TABLE Foo_1337 (bar int)")
	tableID, err := tables.NewTableID("2")
	require.NoError(t, err)
	expReceipt := eventprocessor.Receipt{
		ChainID: chainID,
		TxnHash: txnHash.String(),
		Error:   nil,
		TableID: &tableID,
	}
	require.Eventually(t, checkReceipts(t, expReceipt), time.Second*5, time.Millisecond*100)

	t.Run("transfer", func(t *testing.T) {
		controller := common.HexToAddress("0x39b1b9B439312Dd9E1aE137ce9866e873eA4d211")
		txnHash := contractCalls.transfer(controller)

		tid := tables.TableID(*big.NewInt(1))
		expReceipt := eventprocessor.Receipt{
			ChainID: chainID,
			TxnHash: txnHash.Hex(),
			Error:   nil,
			TableID: &tid,
		}
		require.Eventually(t, checkReceipts(t, expReceipt), time.Second*5, time.Millisecond*100)
	})
}

type contractCalls struct {
	runSQL        contractRunSQLBlockSender
	createTable   contractCreateTableSender
	setController contractSetControllerSender
	transfer      contractTransferFromSender
}

type (
	dbReader                    func(string) []int64
	contractRunSQLBlockSender   func([]string) []common.Hash
	contractCreateTableSender   func(string) common.Hash
	contractSetControllerSender func(controller common.Address) common.Hash
	contractTransferFromSender  func(controller common.Address) common.Hash
	checkReceipts               func(*testing.T, ...eventprocessor.Receipt) func() bool
)

func setup(t *testing.T) (
	contractCalls,
	checkReceipts,
	dbReader,
) {
	t.Helper()

	// Spin up the EVM chain with the contract.
	backend, addr, sc, authOpts, _ := testutil.Setup(t)

	// Spin up dependencies needed for the EventProcessor.
	// i.e: Executor, Parser, and EventFeed (connected to the EVM chain)
	dbURI := tests.Sqlite3URI(t)
	parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", dbURI)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	ex, err := executor.NewExecutor(chainID, db, parser, 0, &aclMock{})
	require.NoError(t, err)

	systemStore, err := system.New(dbURI, tableland.ChainID(chainID))
	require.NoError(t, err)
	ef, err := efimpl.New(
		systemStore,
		chainID,
		backend,
		addr,
		eventfeed.WithNewHeadPollFreq(time.Millisecond),
		eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)

	// Create EventProcessor for our test.
	ep, err := New(parser, ex, ef, chainID)
	require.NoError(t, err)

	ctx := context.Background()
	contractSendRunSQL := func(queries []string) []common.Hash {
		var txnHashes []common.Hash
		for _, q := range queries {
			txn, err := sc.RunSQL(authOpts, authOpts.From, big.NewInt(1), q)

			require.NoError(t, err)
			txnHashes = append(txnHashes, txn.Hash())
		}
		backend.Commit()
		return txnHashes
	}

	contractSendSetController := func(controller common.Address) common.Hash {
		txn, err := sc.SetController(authOpts, authOpts.From, big.NewInt(1), controller)
		require.NoError(t, err)
		backend.Commit()
		return txn.Hash()
	}

	mintTable := func(query string) common.Hash {
		txn, err := sc.CreateTable(authOpts, authOpts.From, query)
		require.NoError(t, err)
		backend.Commit()
		return txn.Hash()
	}

	transferFrom := func(controller common.Address) common.Hash {
		txn, err := sc.TransferFrom(authOpts, authOpts.From, controller, big.NewInt(1))
		require.NoError(t, err)
		backend.Commit()
		return txn.Hash()
	}

	require.NoError(t, err)
	store, err := system.New(
		dbURI, 1337)
	require.NoError(t, err)

	store.SetReadResolver(rsresolver.New(map[tableland.ChainID]eventprocessor.EventProcessor{chainID: ep}))

	tableReader := func(readQuery string) []int64 {
		rq, err := parser.ValidateReadQuery(readQuery)
		require.NoError(t, err)
		require.NotNil(t, rq)
		res, err := store.Read(ctx, rq)
		require.NoError(t, err)

		ret := make([]int64, len(res.Rows))
		for i := range res.Rows {
			ret[i] = res.Rows[i][0].Value().(int64)
		}
		return ret
	}

	checkReceipts := func(t *testing.T, rs ...eventprocessor.Receipt) func() bool {
		return func() bool {
			for _, expReceipt := range rs {
				gotReceipt, found, err := systemStore.GetReceipt(context.Background(), expReceipt.TxnHash)
				require.NoError(t, err)
				if !found {
					return false
				}
				require.Equal(t, expReceipt.ChainID, gotReceipt.ChainID)
				require.NotZero(t, gotReceipt.BlockNumber)
				require.Equal(t, expReceipt.IndexInBlock, gotReceipt.IndexInBlock)
				require.Equal(t, expReceipt.TxnHash, gotReceipt.TxnHash)
				require.Equal(t, expReceipt.Error, gotReceipt.Error)
				require.Equal(t, expReceipt.TableID, gotReceipt.TableID)
			}
			return true
		}
	}

	_ = mintTable("CREATE TABLE test_1337 (bar int)")

	err = ep.Start()
	require.NoError(t, err)
	t.Cleanup(func() { ep.Stop() })

	return contractCalls{
		runSQL:        contractSendRunSQL,
		createTable:   mintTable,
		setController: contractSendSetController,
		transfer:      transferFrom,
	}, checkReceipts, tableReader
}

type aclMock struct{}

func (acl *aclMock) CheckPrivileges(
	_ context.Context,
	_ *sql.Tx,
	_ common.Address,
	_ tables.TableID,
	_ tableland.Operation,
) (bool, error) {
	return true, nil
}
