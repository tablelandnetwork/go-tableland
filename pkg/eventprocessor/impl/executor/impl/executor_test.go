package impl

import (
	"context"
	"database/sql"
	"math/big"
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
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/tests"
)

var chainID = 1337

func TestReceiptExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ex, _ := newExecutorWithTable(t, 0)

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

func TestMultiEventTxnBlock(t *testing.T) {
	t.Parallel()

	// The test setup is executing a block that has the following transactions:
	// - Txn 1, successful with two events: CREATE a table and INSERT a row.
	// - Txn 2, failed with two events: CREATE a table and INSERT a row. INSERT has invalid SQL.
	// - Txn 3, successful with one event: INSERT a row in the table created in Txn 1.
	//
	// Checks after block execution:
	// 1) Txn 1 side-effects are seen in the db.
	// 2) Txn 2 side-effects aren't seen in the db. In particular, the table created should have been rollbacked.
	// 3) Txn 3 side-effects are seen in the db. Txn 2 failing should be isolated and have no effect in Txn 3 execution.
	ctx := context.Background()
	ex, dbURI := newExecutor(t, 0)

	bs, err := ex.NewBlockScope(ctx, 0)
	require.NoError(t, err)

	// Txn 1
	{
		eventCreateTable := &ethereum.ContractCreateTable{
			TableId:   big.NewInt(100),
			Owner:     common.HexToAddress("0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF"),
			Statement: "create table bar_1337 (zar text)",
		}
		eventInsertRow := &ethereum.ContractRunSQL{
			IsOwner:   true,
			TableId:   eventCreateTable.TableId,
			Statement: "insert into bar_1337_100 values ('txn 1')",
			Policy: ethereum.ITablelandControllerPolicy{
				AllowInsert:      true,
				AllowUpdate:      true,
				AllowDelete:      true,
				WhereClause:      "",
				WithCheck:        "",
				UpdatableColumns: nil,
			},
		}
		res, err := bs.ExecuteTxnEvents(ctx, eventfeed.TxnEvents{
			Events: []interface{}{eventCreateTable, eventInsertRow},
		})
		require.NoError(t, err)
		require.Nil(t, res.Error)
		require.Nil(t, res.ErrorEventIdx)
		require.Equal(t, eventCreateTable.TableId.Int64(), res.TableID.ToBigInt().Int64())
	}
	// Txn 2
	{
		eventCreateTable := &ethereum.ContractCreateTable{
			TableId:   big.NewInt(101),
			Owner:     common.HexToAddress("0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF"),
			Statement: "create table foo_1337 (fooz text)",
		}
		eventInsertRow := &ethereum.ContractRunSQL{
			IsOwner:   true,
			TableId:   eventCreateTable.TableId,
			Statement: "insert into foo_1337 values ('txn 1', 'wrong # of columns')",
			Policy: ethereum.ITablelandControllerPolicy{
				AllowInsert:      true,
				AllowUpdate:      true,
				AllowDelete:      true,
				WhereClause:      "",
				WithCheck:        "",
				UpdatableColumns: nil,
			},
		}
		res, err := bs.ExecuteTxnEvents(ctx, eventfeed.TxnEvents{
			Events: []interface{}{eventCreateTable, eventInsertRow},
		})
		require.NoError(t, err)
		// This Txn should fail.
		require.NotNil(t, res.Error)
		require.Equal(t, 1, *res.ErrorEventIdx)
	}
	// Txn 3
	{
		// We can leverage helper for single event txn.
		assertExecTxnWithRunSQLEvents(t, bs, []string{"insert into bar_1337_100 values ('txn 3')"})
	}
	require.NoError(t, bs.Commit())
	require.NoError(t, bs.Close())

	// Post block-execution checks
	{
		// Check 1) and 3).
		require.True(t, existsTableWithName(t, dbURI, "bar_1337_100"))
		require.Equal(t, 2, tableRowCountT100(t, dbURI, "select count(*) from bar_1337_100"))

		// Check 2).
		require.False(t, existsTableWithName(t, dbURI, "foo_1337_101"))
	}
}

func tableRowCountT100(t *testing.T, dbURI string, query string) int {
	t.Helper()

	db, err := sql.Open("sqlite3", dbURI)
	require.NoError(t, err)

	row := db.QueryRowContext(context.Background(), query)
	var rowCount int
	if err = row.Scan(&rowCount); err == sql.ErrNoRows {
		return 0
	}
	require.NoError(t, err)

	return rowCount
}

func existsTableWithName(t *testing.T, dbURI string, tableName string) bool {
	t.Helper()

	db, err := sql.Open("sqlite3", dbURI)
	require.NoError(t, err)
	q := `SELECT 1 FROM sqlite_master  WHERE type='table' AND name = ?1`
	row := db.QueryRow(q, tableName)
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

	dbURI := tests.Sqlite3URI(t)

	parser := newParser(t, []string{})
	db, err := sql.Open("sqlite3", dbURI)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	exec, err := NewExecutor(1337, db, parser, rowsLimit, &aclMock{})
	require.NoError(t, err)

	// Boostrap system store to run the db migrations.
	_, err = system.New(dbURI, tableland.ChainID(chainID))
	require.NoError(t, err)
	return exec, dbURI
}

func newExecutorWithTable(t *testing.T, rowsLimit int) (*Executor, string) {
	t.Helper()

	ex, dbURI := newExecutor(t, rowsLimit)
	ctx := context.Background()

	ibs, err := ex.NewBlockScope(ctx, 0)
	require.NoError(t, err)
	bs := ibs.(*blockScope)

	// Pre-bake a table with ID 100.
	id, err := tables.NewTableID("100")
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
	require.Nil(t, res.ErrorEventIdx)
	require.NotNil(t, res.TableID)

	require.NoError(t, bs.Commit())
	require.NoError(t, bs.Close())

	return ex, dbURI
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
	_ context.Context,
	_ *sql.Tx,
	_ common.Address,
	_ tables.TableID,
	_ tableland.Operation,
) (bool, error) {
	return true, nil
}
