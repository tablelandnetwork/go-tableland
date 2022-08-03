package impl

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/tests"
)

var chainID = 1337

func TestReceiptExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ex, _, _ := newExecutorWithTable(t, 0)

	txnHash := "0x0000000000000000000000000000000000000000000000000000000000001234"

	bs, err := ex.NewBlockScope(ctx, 0)
	require.NoError(t, err)
	ok, err := bs.TxnReceiptExists(ctx, common.HexToHash(txnHash))
	require.NoError(t, err)
	require.False(t, ok)
	require.NoError(t, bs.Commit())
	require.NoError(t, bs.Close())

	bs, err = ex.NewBlockScope(ctx, 0)
	require.NoError(t, err)
	err = bs.SaveTxnReceipts(ctx, []eventprocessor.Receipt{
		{
			ChainID:     tableland.ChainID(chainID),
			BlockNumber: 100,
			TxnHash:     txnHash,
		},
	})
	require.NoError(t, err)
	require.NoError(t, bs.Commit())
	require.NoError(t, bs.Close())

	bs, err = ex.NewBlockScope(ctx, 0)
	require.NoError(t, err)
	ok, err = bs.TxnReceiptExists(ctx, common.HexToHash(txnHash))
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, bs.Commit())
	require.NoError(t, bs.Close())

	require.NoError(t, ex.Close(ctx))
}

func tableRowCountT100(t *testing.T, pool *sql.DB, query string) int {
	t.Helper()

	row := pool.QueryRowContext(context.Background(), query)
	var rowCount int
	err := row.Scan(&rowCount)
	if err == sql.ErrNoRows {
		return 0
	}
	require.NoError(t, err)

	return rowCount
}

func existsTableWithName(t *testing.T, dbURL string, tableName string) bool {
	t.Helper()

	pool, err := sql.Open("sqlite3", dbURL)
	require.NoError(t, err)
	q := `SELECT 1 FROM sqlite_master  WHERE type='table' AND name = ?1`
	row := pool.QueryRow(q, tableName)
	var dummy int
	err = row.Scan(&dummy)
	if err == sql.ErrNoRows {
		return false
	}
	require.NoError(t, err)
	return true
}

func newExecutor(t *testing.T, rowsLimit int) (*Executor, string) {
	t.Helper()

	dbURI := tests.Sqlite3URI()

	parser := newParser(t, []string{})
	exec, err := NewExecutor(1337, dbURI, parser, rowsLimit, &aclMock{})
	require.NoError(t, err)

	// Boostrap system store to run the db migrations.
	_, err = system.New(dbURI, tableland.ChainID(chainID))
	require.NoError(t, err)
	return exec, dbURI
}

func newExecutorWithTable(t *testing.T, rowsLimit int) (*Executor, string, *sql.DB) {
	t.Helper()

	ex, dbURL := newExecutor(t, rowsLimit)
	ctx := context.Background()

	ibs, err := ex.NewBlockScope(ctx, 0)
	require.NoError(t, err)
	bs := ibs.(*blockScope)

	// Pre-bake a table with ID 100.
	id, err := tableland.NewTableID("100")
	require.NoError(t, err)
	require.NoError(t, err)
	res, err := bs.ExecuteTxnEvents(ctx, eventfeed.TxnEvents{
		TxnHash: common.HexToHash("0xF1"),
		Events: []interface{}{
			&ethereum.ContractCreateTable{
				Owner:     common.HexToAddress("0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF"),
				TableId:   id.ToBigInt(),
				Statement: "create table foo_1337 (zar text)",
			},
		},
	})
	require.NoError(t, err)
	require.Nil(t, res.Error)
	require.NotNil(t, res.TableID)

	require.NoError(t, bs.Commit())
	require.NoError(t, bs.Close())

	pool, err := sql.Open("sqlite3", dbURL)
	require.NoError(t, err)

	return ex, dbURL, pool
}

func mustGrantStmt(t *testing.T, q string) parsing.MutatingStmt {
	t.Helper()
	p := newParser(t, []string{"system_", "registry"})
	wss, err := p.ValidateMutatingQuery(q, 1337)
	require.NoError(t, err)
	require.Len(t, wss, 1)
	return wss[0]
}

func newParser(t *testing.T, prefixes []string) parsing.SQLValidator {
	t.Helper()
	p, err := parserimpl.New(prefixes)
	require.NoError(t, err)
	return p
}

type aclMock struct{}

func (acl *aclMock) CheckPrivileges(
	ctx context.Context,
	tx *sql.Tx,
	controller common.Address,
	id tableland.TableID,
	op tableland.Operation,
) (bool, error) {
	return true, nil
}

// TODO(jsign) HIGH: Simulate block with txn with multiple events.
