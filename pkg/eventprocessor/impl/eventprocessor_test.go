package impl

import (
	"context"
	"math/big"
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
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/testutil"
	txnpimpl "github.com/textileio/go-tableland/pkg/txn/impl"
	"github.com/textileio/go-tableland/tests"
)

const chainID = 1337

func TestBlockProcessing(t *testing.T) {
	t.Parallel()

	tableID, err := tableland.NewTableID("1")
	require.NoError(t, err)

	expWrongTypeErr := "db query execution failed (code: POSTGRES_22P02, msg: ERROR: invalid input syntax for type integer: \"abc\" (SQLSTATE 22P02))" //nolint
	cond := func(dr dbReader, exp []int) func() bool {
		return func() bool {
			got := dr("select * from test_1")
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

		contractSendRunSQL, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1 values (1001)"}
		txnHashes := contractSendRunSQL(queries)

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

		contractSendRunSQL, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1 values ('abc')"}
		txnHashes := contractSendRunSQL(queries)

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
		contractSendRunSQL, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1 values (1001)", "insert into test_1 values (1002)"}
		txnHashes := contractSendRunSQL(queries)

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
		contractSendRunSQL, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1 values ('abc')", "insert into test_1 values (1002)"}
		txnHashes := contractSendRunSQL(queries)

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
		contractSendRunSQL, checkReceipts, dbReader := setup(t)
		queries := []string{"insert into test_1 values (1001)", "insert into test_1 values ('abc')"}
		txnHashes := contractSendRunSQL(queries)

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

type dbReader func(string) []int
type contractRunSQLBlockSender func([]string) []common.Hash
type checkReceipts func(*testing.T, ...eventprocessor.Receipt) func() bool

func setup(t *testing.T) (contractRunSQLBlockSender, checkReceipts, dbReader) {
	t.Helper()

	// Spin up the EVM chain with the contract.
	backend, addr, sc, authOpts, _ := testutil.Setup(t)

	// Spin up dependencies needed for the EventProcessor.
	// i.e: TxnProcessor, Parser, and EventFeed (connected to the EVM chain)
	ef, err := efimpl.New(backend, addr, eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)
	url := tests.PostgresURL(t)
	txnp, err := txnpimpl.NewTxnProcessor(url, 0, &aclMock{})
	require.NoError(t, err)
	parser := parserimpl.New([]string{"system_", "registry"}, 0, 0)

	// Create EventProcessor for our test.
	ep, err := New(parser, txnp, ef, chainID)
	require.NoError(t, err)

	ctx := context.Background()
	contractSendRunSQL := func(queries []string) []common.Hash {
		txnHashes := make([]common.Hash, len(queries))
		for i, q := range queries {
			txn, err := sc.RunSQL(authOpts, "1", common.HexToAddress("0xdeadbeef"), q)
			require.NoError(t, err)
			txnHashes[i] = txn.Hash()
		}
		backend.Commit()
		return txnHashes
	}

	sqlstr, err := sqlstoreimpl.New(ctx, url)
	require.NoError(t, err)
	tableReader := func(readQuery string) []int {
		rq, _, err := parser.ValidateRunSQL(readQuery)
		require.NoError(t, err)
		require.NotNil(t, rq)
		res, err := sqlstr.Read(ctx, rq)
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
				gotReceipt, found, err := sqlstr.GetReceipt(context.Background(), chainID, expReceipt.TxnHash)
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

	createStmt, err := parser.ValidateCreateTable("CREATE TABLE test (foo int)")
	require.NoError(t, err)
	err = b.InsertTable(ctx, tableland.TableID(*big.NewInt(1)), "ctrl-1", "descrp-1", createStmt)
	require.NoError(t, err)
	err = b.Commit(ctx)
	require.NoError(t, err)
	err = b.Close(ctx)
	require.NoError(t, err)

	err = ep.Start()
	require.NoError(t, err)
	t.Cleanup(func() { ep.Stop() })

	return contractSendRunSQL, checkReceipts, tableReader
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

func (acl *aclMock) CheckAuthorization(ctx context.Context, controller common.Address) error {
	return nil
}

func (acl *aclMock) IsOwner(ctx context.Context, controller common.Address, id tableland.TableID) (bool, error) {
	return true, nil
}
