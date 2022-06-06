package impl

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/user"
	"github.com/textileio/go-tableland/pkg/tables/impl/testutil"
	txnpimpl "github.com/textileio/go-tableland/pkg/txn/impl"
	"github.com/textileio/go-tableland/tests"
)

const chainID = 1337

func TestRunSQLBlockProcessing(t *testing.T) {
	t.Parallel()

	tableID, err := tableland.NewTableID("1000")
	require.NoError(t, err)

	expWrongTypeErr := "db query execution failed (code: POSTGRES_22P02, msg: ERROR: invalid input syntax for type integer: \"abc\" (SQLSTATE 22P02))" // nolint
	cond := func(dr dbReader, exp []int) func() bool {
		return func() bool {
			got := dr("select * from test_1337_1000")
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
		queries := []string{"insert into test_1337_1000 values (1001)"}
		txnHashes := contractCalls.runSQL(queries)

		expReceipt := eventprocessor.Receipt{
			ChainID: chainID,
			TxnHash: txnHashes[0].String(),
			Error:   nil,
			TableID: &tableID,
		}
		require.Eventually(t, checkReceipts(t, expReceipt), time.Second*5, time.Millisecond*100)

		expectedRows := []int{1001}
		require.Eventually(t, cond(dbReader, expectedRows), time.Second, time.Millisecond*100)
	})
	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		contractCalls, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1337_1000 values ('abc')"}
		txnHashes := contractCalls.runSQL(queries)

		expReceipt := eventprocessor.Receipt{
			ChainID: chainID,
			TxnHash: txnHashes[0].String(),
			Error:   &expWrongTypeErr,
			TableID: nil,
		}
		require.Eventually(t, checkReceipts(t, expReceipt), time.Second*5, time.Millisecond*100)

		notExpectedRows := []int{1001}
		require.Never(t, cond(dbReader, notExpectedRows), time.Second, time.Millisecond*100)
	})
	t.Run("success-success", func(t *testing.T) {
		t.Parallel()

		contractCalls, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1337_1000 values (1001)", "insert into test_1337_1000 values (1002)"}
		txnHashes := contractCalls.runSQL(queries)

		expReceipts := make([]eventprocessor.Receipt, len(txnHashes))
		for i, th := range txnHashes {
			expReceipts[i] = eventprocessor.Receipt{
				ChainID: chainID,
				TxnHash: th.String(),
				Error:   nil,
				TableID: &tableID,
			}
		}
		require.Eventually(t, checkReceipts(t, expReceipts...), time.Second*5, time.Millisecond*100)

		expectedRows := []int{1001, 1002}
		require.Eventually(t, cond(dbReader, expectedRows), time.Second, time.Millisecond*100)
	})
	t.Run("failure-success", func(t *testing.T) {
		t.Parallel()

		contractCalls, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1337_1000 values ('abc')", "insert into test_1337_1000 values (1002)"}
		txnHashes := contractCalls.runSQL(queries)

		expReceipts := make([]eventprocessor.Receipt, len(txnHashes))
		expReceipts[0] = eventprocessor.Receipt{
			ChainID: chainID,
			TxnHash: txnHashes[0].String(),
			Error:   &expWrongTypeErr,
			TableID: nil,
		}
		expReceipts[1] = eventprocessor.Receipt{
			ChainID: chainID,
			TxnHash: txnHashes[1].String(),
			Error:   nil,
			TableID: &tableID,
		}
		require.Eventually(t, checkReceipts(t, expReceipts...), time.Second*5, time.Millisecond*100)

		expectedRows := []int{1002}
		require.Eventually(t, cond(dbReader, expectedRows), time.Second, time.Millisecond*100)
	})
	t.Run("success-failure", func(t *testing.T) {
		t.Parallel()
		contractCalls, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1337_1000 values (1001)", "insert into test_1337_1000 values ('abc')"}
		txnHashes := contractCalls.runSQL(queries)

		expReceipts := make([]eventprocessor.Receipt, len(txnHashes))
		expReceipts[0] = eventprocessor.Receipt{
			ChainID: chainID,
			TxnHash: txnHashes[0].String(),
			Error:   nil,
			TableID: &tableID,
		}
		expReceipts[1] = eventprocessor.Receipt{
			ChainID: chainID,
			TxnHash: txnHashes[1].String(),
			Error:   &expWrongTypeErr,
			TableID: nil,
		}
		require.Eventually(t, checkReceipts(t, expReceipts...), time.Second*5, time.Millisecond*100)

		expectedRows := []int{1001}
		require.Eventually(t, cond(dbReader, expectedRows), time.Second, time.Millisecond*100)
	})
}

func TestCreateTableBlockProcessing(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		contractCalls, checkReceipts, _ := setup(t)
		for i := 0; i < 2; i++ {
			txnHash := contractCalls.createTable("CREATE TABLE Foo_1337 (bar bigint)")

			tableID, err := tableland.NewTableID(strconv.Itoa(i + 1))
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

	expWrongTypeErr := "query validation: unable to parse the query: syntax error at or near \"CREATEZ\""
	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		contractCalls, checkReceipts, _ := setup(t)
		txnHash := contractCalls.createTable("CREATEZ TABLE Foo_1337 (bar bigint)")

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
	// Note that we make a query for table 9999 instead of 1000 which was
	// provided in the SC runSQL call.
	queries := []string{"insert into test_1337_9999 values (1001)"}
	txnHashes := contractCalls.runSQL(queries)

	expErr := "query targets table id 9999 and not 1000"
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
	txnHash := contractCalls.createTable("CREATE TABLE Foo_1337 (bar bigint)")
	tableID, err := tableland.NewTableID("1")
	require.NoError(t, err)
	expReceipt := eventprocessor.Receipt{
		ChainID: chainID,
		TxnHash: txnHash.String(),
		Error:   nil,
		TableID: &tableID,
	}
	require.Eventually(t, checkReceipts(t, expReceipt), time.Second*5, time.Millisecond*100)

	t.Run("set-controller", func(t *testing.T) {
		controller := common.HexToAddress("0x39b1b9B439312Dd9E1aE137ce9866e873eA4d211")
		txnHash := contractCalls.setController(controller)

		tid := tableland.TableID(*big.NewInt(1))
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
		fmt.Println(controller.Hex())
		txnHash := contractCalls.setController(controller)

		tid := tableland.TableID(*big.NewInt(1))
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

	txnHash := contractCalls.createTable("CREATE TABLE Foo_1337 (bar bigint)")
	tableID, err := tableland.NewTableID("1")
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

		tid := tableland.TableID(*big.NewInt(1))
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

type dbReader func(string) []int
type contractRunSQLBlockSender func([]string) []common.Hash
type contractCreateTableSender func(string) common.Hash
type contractSetControllerSender func(controller common.Address) common.Hash
type contractTransferFromSender func(controller common.Address) common.Hash
type checkReceipts func(*testing.T, ...eventprocessor.Receipt) func() bool

func setup(t *testing.T) (
	contractCalls,
	checkReceipts,
	dbReader) {
	t.Helper()

	// Spin up the EVM chain with the contract.
	backend, addr, sc, authOpts, _ := testutil.Setup(t)

	// Spin up dependencies needed for the EventProcessor.
	// i.e: TxnProcessor, Parser, and EventFeed (connected to the EVM chain)
	ef, err := efimpl.New(chainID, backend, addr, eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)
	url := tests.PostgresURL(t)
	txnp, err := txnpimpl.NewTxnProcessor(chainID, url, 0, &aclMock{})
	require.NoError(t, err)
	parser, err := parserimpl.New([]string{"system_", "registry"})
	require.NoError(t, err)

	// Create EventProcessor for our test.
	ep, err := New(parser, txnp, ef, chainID)
	require.NoError(t, err)

	ctx := context.Background()
	contractSendRunSQL := func(queries []string) []common.Hash {
		txnHashes := make([]common.Hash, len(queries))
		for i, q := range queries {
			txn, err := sc.RunSQL(authOpts, authOpts.From, big.NewInt(1000), q)

			require.NoError(t, err)
			txnHashes[i] = txn.Hash()
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

	systemStore, err := system.New(url, tableland.ChainID(chainID))

	transferFrom := func(controller common.Address) common.Hash {
		txn, err := sc.TransferFrom(authOpts, authOpts.From, controller, big.NewInt(1))
		require.NoError(t, err)
		backend.Commit()
		return txn.Hash()
	}

	require.NoError(t, err)
	userStore, err := user.New(url)
	require.NoError(t, err)

	tableReader := func(readQuery string) []int {
		rq, err := parser.ValidateReadQuery(readQuery)
		require.NoError(t, err)
		require.NotNil(t, rq)
		res, err := userStore.Read(ctx, rq)
		require.NoError(t, err)

		queryRes := res.(sqlstore.UserRows)
		ret := make([]int, len(queryRes.Rows))
		for i := range queryRes.Rows {
			ret[i] = **queryRes.Rows[i][0].(**int)
		}
		return ret
	}

	checkReceipts := func(t *testing.T, rs ...eventprocessor.Receipt) func() bool {
		return func() bool {
			for _, expReceipt := range rs {
				gotReceipt, found, err := systemStore.GetReceipt(context.Background(), expReceipt.TxnHash)
				if !found {
					return false
				}
				require.NoError(t, err)
				require.Equal(t, expReceipt.ChainID, gotReceipt.ChainID)
				require.NotZero(t, gotReceipt.BlockNumber)
				require.Equal(t, expReceipt.TxnHash, gotReceipt.TxnHash)
				require.Equal(t, expReceipt.Error, gotReceipt.Error)
				require.Equal(t, expReceipt.TableID, gotReceipt.TableID)
			}
			return true
		}
	}

	b, err := txnp.OpenBatch(ctx)
	require.NoError(t, err)

	createStmt, err := parser.ValidateCreateTable("CREATE TABLE test_1337 (foo int)", chainID)
	require.NoError(t, err)
	err = b.InsertTable(ctx, tableland.TableID(*big.NewInt(1000)), "ctrl-1", createStmt)
	require.NoError(t, err)
	err = b.Commit(ctx)
	require.NoError(t, err)
	err = b.Close(ctx)
	require.NoError(t, err)

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
	ctx context.Context,
	tx pgx.Tx,
	controller common.Address,
	id tableland.TableID,
	op tableland.Operation) (bool, error) {
	return true, nil
}
