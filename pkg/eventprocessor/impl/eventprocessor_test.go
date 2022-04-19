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
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/testutil"
	txnpimpl "github.com/textileio/go-tableland/pkg/txn/impl"
	"github.com/textileio/go-tableland/tests"
)

func TestBlockProcessing(t *testing.T) {
	t.Parallel()

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

		contractSendRunSQL, dbReader := setup(t)
		queries := []string{"insert into test_1 values (1001)"}
		contractSendRunSQL(queries)

		expectedRows := []int{1001}
		require.Eventually(t, cond(dbReader, expectedRows), time.Second*5, time.Millisecond*100)
	})
	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		contractSendRunSQL, dbReader := setup(t)
		queries := []string{"insert into test_1 values ('wrongtype1001')"}
		contractSendRunSQL(queries)

		notExpectedRows := []int{1001}
		require.Never(t, cond(dbReader, notExpectedRows), time.Second*5, time.Millisecond*100)
	})
	t.Run("success-success", func(t *testing.T) {
		t.Parallel()
		contractSendRunSQL, dbReader := setup(t)
		queries := []string{"insert into test_1 values (1001)", "insert into test_1 values (1002)"}
		contractSendRunSQL(queries)

		expectedRows := []int{1001, 1002}
		require.Eventually(t, cond(dbReader, expectedRows), time.Second*5, time.Millisecond*100)
	})
	t.Run("failure-success", func(t *testing.T) {
		t.Parallel()
		contractSendRunSQL, dbReader := setup(t)
		queries := []string{"insert into test_1 values ('abc')", "insert into test_1 values (1002)"}
		contractSendRunSQL(queries)

		expectedRows := []int{1002}
		require.Eventually(t, cond(dbReader, expectedRows), time.Second*5, time.Millisecond*100)
	})
	t.Run("success-failure", func(t *testing.T) {
		t.Parallel()
		contractSendRunSQL, dbReader := setup(t)
		queries := []string{"insert into test_1 values (1001)", "insert into test_1 values ('abc')"}
		contractSendRunSQL(queries)

		expectedRows := []int{1001}
		require.Eventually(t, cond(dbReader, expectedRows), time.Second*5, time.Millisecond*100)
	})
}

type dbReader func(string) []int
type contractRunSQLBlockSender func([]string)

func setup(t *testing.T) (contractRunSQLBlockSender, dbReader) {
	t.Helper()

	// Spin up the EVM chain with the contract.
	backend, addr, sc, authOpts, _ := testutil.Setup(t)

	// Spin up dependencies needed for the EventProcessor.
	// i.e: TxnProcessor, Parser, and EventFeed (connected to the EVM chain)
	ef, err := efimpl.New(backend, addr, eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)
	url, err := tests.PostgresURL()
	require.NoError(t, err)
	txnp, err := txnpimpl.NewTxnProcessor(url, 0, &aclMock{})
	require.NoError(t, err)
	parser := parserimpl.New([]string{"system_", "registry"}, 0, 0)

	// Create EventProcessor for our test.
	ep, err := New(parser, txnp, ef)
	require.NoError(t, err)

	ctx := context.Background()
	contractSendRunSQL := func(queries []string) {
		for _, q := range queries {
			_, err := sc.RunSQL(authOpts, "1", common.HexToAddress("0xdeadbeef"), q)
			require.NoError(t, err)
		}
		backend.Commit()
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

	return contractSendRunSQL, tableReader
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
