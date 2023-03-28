package impl

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/gateway"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	epimpl "github.com/textileio/go-tableland/pkg/eventprocessor/impl"
	executor "github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor/impl"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"

	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/tables/impl/testutil"
	"github.com/textileio/go-tableland/pkg/wallet"
	"github.com/textileio/go-tableland/tests"
)

func TestTodoAppWorkflow(t *testing.T) {
	t.Parallel()

	setup := newTablelandSetupBuilder().
		build(t)
	tablelandClient := setup.newTablelandClient(t)

	ctx, backend, sc := setup.ctx, setup.ethClient, setup.contract
	gateway, txOpts := tablelandClient.gateway, tablelandClient.txOpts

	caller := txOpts.From
	_, err := sc.CreateTable(txOpts, caller,
		`CREATE TABLE todoapp_1337 (
			complete INTEGER DEFAULT 0,
			name     TEXT DEFAULT '',
			deleted  INTEGER DEFAULT 0,
			id       INTEGER
		  );`)
	require.NoError(t, err)

	processCSV(ctx, t, sc, txOpts, caller, gateway, "testdata/todoapp_queries.csv", backend)
}

func TestInsertOnConflict(t *testing.T) {
	t.Parallel()
	// TODO: This test was passing because the "DO UPDATE SET" clause didn't have a table name.
	//       It's disabled temporarily until some soon related work in the validator will fix this.
	t.SkipNow()

	setup := newTablelandSetupBuilder().
		build(t)
	tablelandClient := setup.newTablelandClient(t)

	ctx, backend, sc, store := setup.ctx, setup.ethClient, setup.contract, setup.systemStore
	gateway, txOpts := tablelandClient.gateway, tablelandClient.txOpts

	caller := txOpts.From

	_, err := sc.CreateTable(txOpts, caller,
		`CREATE TABLE foo_1337 (
			name text unique,
			count int);`)
	require.NoError(t, err)
	backend.Commit()

	var txnHashes []string
	for i := 0; i < 10; i++ {
		txn, err := sc.RunSQL(
			txOpts,
			caller,
			big.NewInt(1),
			`INSERT INTO foo_1337_1 VALUES ('bar', 0) ON CONFLICT (name) DO UPDATE SET count=_1.count+1`,
		)
		require.NoError(t, err)
		backend.Commit()
		txnHashes = append(txnHashes, txn.Hash().Hex())
	}

	require.Eventually(
		t,
		jsonEq(ctx, t, gateway, "SELECT count FROM foo_1337_1", `{"columns":[{"name":"count"}],"rows":[[9]]}`),
		time.Second*5,
		time.Millisecond*100,
	)
	requireReceipts(ctx, t, store, txnHashes, true)
}

func TestMultiStatement(t *testing.T) {
	t.Parallel()

	setup := newTablelandSetupBuilder().
		build(t)
	tablelandClient := setup.newTablelandClient(t)

	ctx, backend, sc, store := setup.ctx, setup.ethClient, setup.contract, setup.systemStore
	gateway, txOpts := tablelandClient.gateway, tablelandClient.txOpts
	caller := txOpts.From

	_, err := sc.CreateTable(txOpts, caller,
		`CREATE TABLE foo_1337 (
			name text unique
		);`)
	require.NoError(t, err)

	r, err := sc.RunSQL(
		txOpts,
		caller,
		big.NewInt(1),
		`INSERT INTO foo_1337_1 values ('bar'); UPDATE foo_1337_1 SET name='zoo'`,
	)

	require.NoError(t, err)
	backend.Commit()

	require.Eventually(
		t,
		jsonEq(ctx, t, gateway, "SELECT name from foo_1337_1", `{"columns":[{"name":"name"}],"rows":[["zoo"]]}`),
		time.Second*5,
		time.Millisecond*100,
	)
	requireReceipts(ctx, t, store, []string{r.Hash().Hex()}, true)
}

func TestReadSystemTable(t *testing.T) {
	t.Parallel()

	setup := newTablelandSetupBuilder().
		build(t)
	tablelandClient := setup.newTablelandClient(t)

	ctx, sc := setup.ctx, setup.contract
	gateway, txOpts := tablelandClient.gateway, tablelandClient.txOpts
	caller := txOpts.From

	_, err := sc.CreateTable(txOpts, caller, `CREATE TABLE foo_1337 (myjson TEXT);`)
	require.NoError(t, err)

	res, err := runReadQuery(ctx, t, gateway, "select * from registry")
	require.NoError(t, err)
	_, err = json.Marshal(res)
	require.NoError(t, err)
}

func TestJSON(t *testing.T) {
	t.Parallel()

	setup := newTablelandSetupBuilder().
		build(t)
	tablelandClient := setup.newTablelandClient(t)

	ctx, backend, sc := setup.ctx, setup.ethClient, setup.contract
	gateway, txOpts := tablelandClient.gateway, tablelandClient.txOpts
	caller := txOpts.From

	_, err := sc.CreateTable(txOpts, caller, `CREATE TABLE foo_1337 (myjson TEXT);`)
	require.NoError(t, err)

	processCSV(ctx, t, sc, txOpts, caller, gateway, "testdata/json_queries.csv", backend)
}

func TestCheckInsertPrivileges(t *testing.T) {
	t.Parallel()

	type testCase struct { // nolint
		query      string
		privileges tableland.Privileges
		isAllowed  bool
	}

	tests := []testCase{
		{"INSERT INTO foo_1337_1 (bar) VALUES ('Hello')", tableland.Privileges{}, false},
		{"INSERT INTO foo_1337_1 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivInsert}, true},
		{"INSERT INTO foo_1337_1 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivUpdate}, false},
		{"INSERT INTO foo_1337_1 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivDelete}, false},
		{"INSERT INTO foo_1337_1 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate}, true},                       // nolint
		{"INSERT INTO foo_1337_1 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivInsert, tableland.PrivDelete}, true},                       // nolint
		{"INSERT INTO foo_1337_1 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivUpdate, tableland.PrivDelete}, false},                      // nolint
		{"INSERT INTO foo_1337_1 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate, tableland.PrivDelete}, true}, // nolint
	}

	for i, test := range tests {
		t.Run(fmt.Sprint(i+1), func(test testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()

				setup := newTablelandSetupBuilder().
					build(t)

				granterSetup := setup.newTablelandClient(t)
				granteeSetup := setup.newTablelandClient(t)

				ctx, backend, sc, store := setup.ctx, setup.ethClient, setup.contract, setup.systemStore
				txOptsGranter := granterSetup.txOpts
				gatewayGrantee, txOptsGrantee := granteeSetup.gateway, granteeSetup.txOpts

				granter := txOptsGranter.From
				grantee := txOptsGrantee.From

				_, err := sc.CreateTable(txOptsGranter, granter, `CREATE TABLE foo_1337 (bar text);`)
				require.NoError(t, err)
				backend.Commit()

				var successfulTxnHashes []string
				if len(test.privileges) > 0 {
					privileges := make([]string, len(test.privileges))
					for i, priv := range test.privileges {
						privileges[i] = priv.ToSQLString()
					}

					// execute grant statement according to test case
					grantQuery := fmt.Sprintf("GRANT %s ON foo_1337_1 TO '%s'", strings.Join(privileges, ","), grantee)
					txn, err := helpTestWriteQuery(t, sc, txOptsGranter, granter, grantQuery)
					require.NoError(t, err)
					backend.Commit()
					successfulTxnHashes = append(successfulTxnHashes, txn.Hash().Hex())
				}

				txn, err := helpTestWriteQuery(t, sc, txOptsGrantee, grantee, test.query)
				require.NoError(t, err)
				backend.Commit()

				testQuery := "SELECT * FROM foo_1337_1 WHERE bar ='Hello';"
				if test.isAllowed {
					require.Eventually(t,
						runSQLCountEq(ctx, t, gatewayGrantee, testQuery, 1),
						5*time.Second,
						100*time.Millisecond,
					)
					successfulTxnHashes = append(successfulTxnHashes, txn.Hash().Hex())
					requireReceipts(ctx, t, store, successfulTxnHashes, true)
				} else {
					require.Never(t, runSQLCountEq(ctx, t, gatewayGrantee, testQuery, 1), 5*time.Second, 100*time.Millisecond)
					requireReceipts(ctx, t, store, successfulTxnHashes, true)
					requireReceipts(ctx, t, store, []string{txn.Hash().Hex()}, false)
				}
			}
		}(test))
	}
}

func TestCheckUpdatePrivileges(t *testing.T) {
	t.Parallel()

	type testCase struct { // nolint
		query      string
		privileges tableland.Privileges
		isAllowed  bool
	}

	tests := []testCase{
		{"UPDATE foo_1337_1 SET bar = 'Hello 2'", tableland.Privileges{}, false},
		{"UPDATE foo_1337_1 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivInsert}, false},
		{"UPDATE foo_1337_1 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivUpdate}, true},
		{"UPDATE foo_1337_1 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivDelete}, false},
		{"UPDATE foo_1337_1 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate}, true},
		{"UPDATE foo_1337_1 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivInsert, tableland.PrivDelete}, false},
		{"UPDATE foo_1337_1 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivUpdate, tableland.PrivDelete}, true},
		{"UPDATE foo_1337_1 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate, tableland.PrivDelete}, true}, // nolint
	}

	for i, test := range tests {
		t.Run(fmt.Sprint(i+1), func(test testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()

				setup := newTablelandSetupBuilder().
					build(t)

				granterSetup := setup.newTablelandClient(t)
				granteeSetup := setup.newTablelandClient(t)

				ctx, backend, sc, store := setup.ctx, setup.ethClient, setup.contract, setup.systemStore
				txOptsGranter := granterSetup.txOpts
				gatewayGrantee, txOptsGrantee := granteeSetup.gateway, granteeSetup.txOpts

				granter := txOptsGranter.From
				grantee := txOptsGrantee.From

				_, err := sc.CreateTable(txOptsGranter, granter, `CREATE TABLE foo_1337 (bar text);`)
				require.NoError(t, err)
				backend.Commit()
				var successfulTxnHashes []string

				// we initilize the table with a row to be updated
				txn, err := helpTestWriteQuery(t, sc, txOptsGranter, granter, "INSERT INTO foo_1337_1 (bar) VALUES ('Hello')")
				require.NoError(t, err)
				backend.Commit()
				successfulTxnHashes = append(successfulTxnHashes, txn.Hash().Hex())

				if len(test.privileges) > 0 {
					privileges := make([]string, len(test.privileges))
					for i, priv := range test.privileges {
						privileges[i] = priv.ToSQLString()
					}

					// execute grant statement according to test case
					grantQuery := fmt.Sprintf("GRANT %s ON foo_1337_1 TO '%s'", strings.Join(privileges, ","), grantee)
					txn, err := helpTestWriteQuery(t, sc, txOptsGranter, granter, grantQuery)
					require.NoError(t, err)
					backend.Commit()
					successfulTxnHashes = append(successfulTxnHashes, txn.Hash().Hex())
				}

				txn, err = helpTestWriteQuery(t, sc, txOptsGrantee, grantee, test.query)
				require.NoError(t, err)
				backend.Commit()

				testQuery := "SELECT * FROM foo_1337_1 WHERE bar='Hello 2';"
				if test.isAllowed {
					require.Eventually(t,
						runSQLCountEq(ctx, t, gatewayGrantee, testQuery, 1),
						5*time.Second,
						100*time.Millisecond,
					)
					successfulTxnHashes = append(successfulTxnHashes, txn.Hash().Hex())
					requireReceipts(ctx, t, store, successfulTxnHashes, true)
				} else {
					require.Never(t, runSQLCountEq(ctx, t, gatewayGrantee, testQuery, 1), 5*time.Second, 100*time.Millisecond)
					requireReceipts(ctx, t, store, successfulTxnHashes, true)
					requireReceipts(ctx, t, store, []string{txn.Hash().Hex()}, false)
				}
			}
		}(test))
	}
}

func TestCheckDeletePrivileges(t *testing.T) {
	t.Parallel()

	type testCase struct { // nolint
		query      string
		privileges tableland.Privileges
		isAllowed  bool
	}

	tests := []testCase{
		{"DELETE FROM foo_1337_1", tableland.Privileges{}, false},
		{"DELETE FROM foo_1337_1", tableland.Privileges{tableland.PrivInsert}, false},
		{"DELETE FROM foo_1337_1", tableland.Privileges{tableland.PrivUpdate}, false},
		{"DELETE FROM foo_1337_1", tableland.Privileges{tableland.PrivDelete}, true},
		{"DELETE FROM foo_1337_1", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate}, false},
		{"DELETE FROM foo_1337_1", tableland.Privileges{tableland.PrivInsert, tableland.PrivDelete}, true},
		{"DELETE FROM foo_1337_1", tableland.Privileges{tableland.PrivUpdate, tableland.PrivDelete}, true},
		{"DELETE FROM foo_1337_1", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate, tableland.PrivDelete}, true}, // nolint
	}

	for i, test := range tests {
		t.Run(fmt.Sprint(i+1), func(test testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()

				setup := newTablelandSetupBuilder().
					build(t)

				granterSetup := setup.newTablelandClient(t)
				granteeSetup := setup.newTablelandClient(t)

				ctx, backend, sc, store := setup.ctx, setup.ethClient, setup.contract, setup.systemStore
				txOptsGranter := granterSetup.txOpts
				gatewayGrantee, txOptsGrantee := granteeSetup.gateway, granteeSetup.txOpts

				granter := txOptsGranter.From
				grantee := txOptsGrantee.From

				_, err := sc.CreateTable(txOptsGranter, granter, `CREATE TABLE foo_1337 (bar text);`)
				require.NoError(t, err)
				var successfulTxnHashes []string

				// we initilize the table with a row to be delete
				_, err = helpTestWriteQuery(t, sc, txOptsGranter, granter, "INSERT INTO foo_1337_1 (bar) VALUES ('Hello')")
				require.NoError(t, err)
				backend.Commit()

				if len(test.privileges) > 0 {
					privileges := make([]string, len(test.privileges))
					for i, priv := range test.privileges {
						privileges[i] = priv.ToSQLString()
					}

					// execute grant statement according to test case
					grantQuery := fmt.Sprintf("GRANT %s ON foo_1337_1 TO '%s'", strings.Join(privileges, ","), grantee)
					txn, err := helpTestWriteQuery(t, sc, txOptsGranter, granter, grantQuery)
					require.NoError(t, err)
					backend.Commit()
					successfulTxnHashes = append(successfulTxnHashes, txn.Hash().Hex())
				}

				txn, err := helpTestWriteQuery(t, sc, txOptsGrantee, grantee, test.query)
				require.NoError(t, err)
				backend.Commit()

				testQuery := "SELECT * FROM foo_1337_1"
				if test.isAllowed {
					require.Eventually(t,
						runSQLCountEq(ctx, t, gatewayGrantee, testQuery, 0),
						5*time.Second,
						100*time.Millisecond,
					)
					successfulTxnHashes = append(successfulTxnHashes, txn.Hash().Hex())
					requireReceipts(ctx, t, store, successfulTxnHashes, true)
				} else {
					require.Never(t, runSQLCountEq(ctx, t, gatewayGrantee, testQuery, 0), 5*time.Second, 100*time.Millisecond)
					requireReceipts(ctx, t, store, []string{txn.Hash().Hex()}, false)
				}
			}
		}(test))
	}
}

func TestOwnerRevokesItsPrivilegeInsideMultipleStatements(t *testing.T) {
	t.Parallel()

	setup := newTablelandSetupBuilder().
		build(t)
	tablelandClient := setup.newTablelandClient(t)

	ctx, backend, sc, store := setup.ctx, setup.ethClient, setup.contract, setup.systemStore
	gateway, txOpts := tablelandClient.gateway, tablelandClient.txOpts
	caller := txOpts.From

	_, err := sc.CreateTable(txOpts, caller, `CREATE TABLE foo_1337 (bar text);`)
	require.NoError(t, err)

	multiStatements := `
		INSERT INTO foo_1337_1 (bar) VALUES ('Hello');
		UPDATE foo_1337_1 SET bar = 'Hello 2';
		REVOKE update ON foo_1337_1 FROM '` + caller.Hex() + `';
		UPDATE foo_1337_1 SET bar = 'Hello 3';
	`
	txn, err := helpTestWriteQuery(t, sc, txOpts, caller, multiStatements)
	require.NoError(t, err)
	backend.Commit()

	testQuery := "SELECT * FROM foo_1337_1;"
	cond := runSQLCountEq(ctx, t, gateway, testQuery, 1)
	require.Never(t, cond, 5*time.Second, 100*time.Millisecond)
	requireReceipts(ctx, t, store, []string{txn.Hash().Hex()}, false)
}

func TestTransferTable(t *testing.T) {
	t.Parallel()

	setup := newTablelandSetupBuilder().
		build(t)

	owner1Setup := setup.newTablelandClient(t)
	owner2Setup := setup.newTablelandClient(t)

	ctx, backend, sc, store := setup.ctx, setup.ethClient, setup.contract, setup.systemStore
	gatewayOwner1, txOptsOwner1 := owner1Setup.gateway, owner1Setup.txOpts
	gatewayOwner2, txOptsOwner2 := owner2Setup.gateway, owner2Setup.txOpts

	_, err := sc.CreateTable(txOptsOwner1, txOptsOwner1.From, `CREATE TABLE foo_1337 (bar text);`)
	require.NoError(t, err)

	// transfer table from owner1 to owner2
	_, err = sc.TransferFrom(txOptsOwner1, txOptsOwner1.From, txOptsOwner2.From, big.NewInt(1))
	require.NoError(t, err)

	// we'll execute one insert with owner1 and one insert with owner2
	query1 := "INSERT INTO foo_1337_1 (bar) VALUES ('Hello')"
	txn1, err := helpTestWriteQuery(t, sc, txOptsOwner1, txOptsOwner1.From, query1)
	require.NoError(t, err)
	backend.Commit()

	query2 := "INSERT INTO foo_1337_1 (bar) VALUES ('Hello2')"
	txn2, err := helpTestWriteQuery(t, sc, txOptsOwner2, txOptsOwner2.From, query2)
	require.NoError(t, err)
	backend.Commit()

	// insert from owner1 will NEVER go through
	require.Never(t,
		runSQLCountEq(ctx, t, gatewayOwner1, "SELECT * FROM foo_1337_1 WHERE bar ='Hello';", 1),
		5*time.Second,
		100*time.Millisecond,
	)
	requireReceipts(ctx, t, store, []string{txn1.Hash().Hex()}, false)

	// insert from owner2 will EVENTUALLY go through
	require.Eventually(t,
		runSQLCountEq(ctx, t, gatewayOwner2, "SELECT * FROM foo_1337_1 WHERE bar ='Hello2';", 1),
		5*time.Second,
		100*time.Millisecond,
	)
	requireReceipts(ctx, t, store, []string{txn2.Hash().Hex()}, true)

	// check registry table new ownership
	require.Eventually(t,
		runSQLCountEq(ctx,
			t,
			gatewayOwner2,
			fmt.Sprintf("SELECT * FROM registry WHERE controller = '%s' AND id = 1 AND chain_id = 1337", txOptsOwner2.From.Hex()), // nolint
			1,
		),
		5*time.Second,
		100*time.Millisecond,
	)
}

func processCSV(
	ctx context.Context,
	t *testing.T,
	sc *ethereum.Contract,
	txOpts *bind.TransactOpts,
	caller common.Address,
	gateway gateway.Gateway,
	csvPath string,
	backend *backends.SimulatedBackend,
) {
	t.Helper()

	records := readCsvFile(t, csvPath)
	for _, record := range records {
		if record[0] == "r" {
			require.Eventually(t, jsonEq(ctx, t, gateway, record[1], record[2]), time.Second*5, time.Millisecond*100)
		} else {
			_, err := sc.RunSQL(txOpts, caller, big.NewInt(1), record[1])
			require.NoError(t, err)
			backend.Commit()
		}
	}
}

func jsonEq(
	ctx context.Context,
	t *testing.T,
	gateway gateway.Gateway,
	stm string,
	expJSON string,
) func() bool {
	return func() bool {
		r, err := gateway.RunReadQuery(ctx, stm)
		// if we get a table undefined error, try again
		if err != nil && strings.Contains(err.Error(), "no such table") {
			return false
		}
		require.NoError(t, err)

		b, err := json.Marshal(r)
		require.NoError(t, err)

		gotJSON := string(b)

		var o1 interface{}
		var o2 interface{}

		err = json.Unmarshal([]byte(expJSON), &o1)
		if err != nil {
			return false
		}
		err = json.Unmarshal([]byte(gotJSON), &o2)
		if err != nil {
			return false
		}

		return reflect.DeepEqual(o1, o2)
	}
}

func runSQLCountEq(
	ctx context.Context,
	t *testing.T,
	tbld tableland.Tableland,
	sql string,
	expCount int,
) func() bool {
	return func() bool {
		response, err := runReadQuery(ctx, t, tbld, sql)
		// if we get a table undefined error, try again
		if err != nil && strings.Contains(err.Error(), "table not found") {
			return false
		}
		require.NoError(t, err)

		responseInBytes, err := json.Marshal(response)
		require.NoError(t, err)

		r := &struct {
			Rows [][]interface{} `json:"rows"`
		}{}

		err = json.Unmarshal(responseInBytes, r)
		require.NoError(t, err)

		return len(r.Rows) == expCount
	}
}

func runReadQuery(
	ctx context.Context,
	t *testing.T,
	tbld tableland.Tableland,
	sql string,
) (interface{}, error) {
	t.Helper()

	return tbld.RunReadQuery(ctx, sql)
}

func helpTestWriteQuery(
	t *testing.T,
	sc *ethereum.Contract,
	txOpts *bind.TransactOpts,
	caller common.Address,
	sql string,
) (tables.Transaction, error) {
	t.Helper()

	return sc.RunSQL(txOpts, caller, big.NewInt(1), sql)
}

func readCsvFile(t *testing.T, filePath string) [][]string {
	t.Helper()

	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("unable to read input file "+filePath, err)
	}
	defer f.Close() // nolint

	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	if err != nil {
		t.Fatalf("unable to parse file as CSV for "+filePath, err)
	}

	return records
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
	aclImpl := NewACL(acl.sqlStore)
	return aclImpl.CheckPrivileges(ctx, tx, controller, id, op)
}

func (acl *aclHalfMock) IsOwner(_ context.Context, _ common.Address, _ tables.TableID) (bool, error) {
	return true, nil
}

func requireReceipts(
	ctx context.Context,
	t *testing.T,
	store *system.SystemStore,
	txnHashes []string,
	ok bool,
) {
	t.Helper()

	for _, txnHash := range txnHashes {
		// TODO: GetReceipt is only used by the tests, we can use system service instead

		receipt, found, err := store.GetReceipt(ctx, txnHash)
		require.NoError(t, err)
		require.True(t, found)
		require.NotNil(t, receipt)
		require.Equal(t, tableland.ChainID(1337), receipt.ChainID)
		require.Equal(t, txnHash, txnHash)
		require.NotZero(t, receipt.BlockNumber)
		if ok {
			require.Empty(t, receipt.Error)
			require.NotNil(t, receipt.TableID)
			require.NotZero(t, receipt.TableID)
		} else {
			require.NotEmpty(t, receipt.Error)
			require.Nil(t, receipt.TableID)
		}
	}
}

func requireTxn(
	t *testing.T,
	backend *backends.SimulatedBackend,
	key *ecdsa.PrivateKey,
	from common.Address,
	to common.Address,
	amt *big.Int,
) {
	nonce, err := backend.PendingNonceAt(context.Background(), from)
	require.NoError(t, err)

	gasLimit := uint64(21000)
	gasPrice, err := backend.SuggestGasPrice(context.Background())
	require.NoError(t, err)

	var data []byte
	txnData := &types.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      gasLimit,
		To:       &to,
		Data:     data,
		Value:    amt,
	}
	tx := types.NewTx(txnData)
	signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, key)
	require.NoError(t, err)

	bal, err := backend.BalanceAt(context.Background(), from, nil)
	require.NoError(t, err)
	require.NotZero(t, bal)

	err = backend.SendTransaction(context.Background(), signedTx)
	require.NoError(t, err)

	backend.Commit()

	receipt, err := backend.TransactionReceipt(context.Background(), signedTx.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)
}

type tablelandSetupBuilder struct {
	parsingOpts []parsing.Option
}

func newTablelandSetupBuilder() *tablelandSetupBuilder {
	return &tablelandSetupBuilder{}
}

func (b *tablelandSetupBuilder) build(t *testing.T) *tablelandSetup {
	t.Helper()
	dbURI := tests.Sqlite3URI(t)

	ctx := context.Background()
	store, err := system.New(dbURI, tableland.ChainID(1337))
	require.NoError(t, err)

	parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"}, b.parsingOpts...)
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", dbURI)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)

	ex, err := executor.NewExecutor(1337, db, parser, 0, &aclHalfMock{store})
	require.NoError(t, err)

	backend, addr, sc, auth, sk := testutil.Setup(t)

	// Spin up dependencies needed for the EventProcessor.
	// i.e: Executor, Parser, and EventFeed (connected to the EVM chain)
	ef, err := efimpl.New(store,
		1337,
		backend,
		addr,
		eventfeed.WithNewHeadPollFreq(time.Millisecond),
		eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)

	// Create EventProcessor for our test.
	ep, err := epimpl.New(parser, ex, ef, 1337)
	require.NoError(t, err)

	err = ep.Start()
	require.NoError(t, err)
	t.Cleanup(func() { ep.Stop() })

	return &tablelandSetup{
		ctx: ctx,

		chainID: 1337,

		// ethereum client
		ethClient: backend,

		// contract bindings
		contract:     sc,
		contractAddr: addr,

		// contract deployer
		deployerPrivateKey: sk,
		deployerTxOpts:     auth,

		// common dependencies among mesa clients
		parser:      parser,
		systemStore: store,
	}
}

type tablelandSetup struct {
	ctx context.Context

	chainID tableland.ChainID

	// ethereum client
	ethClient *backends.SimulatedBackend

	// contract bindings
	contract     *ethereum.Contract
	contractAddr common.Address

	// contract deployer
	deployerPrivateKey *ecdsa.PrivateKey
	deployerTxOpts     *bind.TransactOpts

	// common dependencies among tableland clients
	parser      parsing.SQLValidator
	systemStore *system.SystemStore
}

func (s *tablelandSetup) newTablelandClient(t *testing.T) *tablelandClient {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	wallet, err := wallet.NewWallet(hex.EncodeToString(crypto.FromECDSA(key)))
	require.NoError(t, err)

	txOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337)) // nolint
	require.NoError(t, err)

	requireTxn(t,
		s.ethClient,
		s.deployerPrivateKey,
		s.deployerTxOpts.From,
		wallet.Address(),
		big.NewInt(1000000000000000000),
	)

	gateway, err := gateway.NewGateway(
		s.parser,
		map[tableland.ChainID]sqlstore.SystemStore{1337: s.systemStore},
		"https://tableland.network/tables",
		"https://render.tableland.xyz",
		"https://render.tableland.xyz/anim",
	)
	require.NoError(t, err)

	return &tablelandClient{
		gateway: gateway,
		txOpts:  txOpts,
	}
}

type tablelandClient struct {
	gateway gateway.Gateway
	txOpts  *bind.TransactOpts
}
