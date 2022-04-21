package impl

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v4"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	epimpl "github.com/textileio/go-tableland/pkg/eventprocessor/impl"
	"github.com/textileio/go-tableland/pkg/nonce/impl"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/testutil"
	txnpimpl "github.com/textileio/go-tableland/pkg/txn/impl"
	"github.com/textileio/go-tableland/pkg/wallet"
	"github.com/textileio/go-tableland/tests"
)

func init() {
	log = log.Level(zerolog.ErrorLevel)
}

func TestTodoAppWorkflow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tbld, backend, sc, auth := setup(ctx, t)

	caller := common.HexToAddress("0xd43c59d5694ec111eb9e986c233200b14249558d")
	_, err := sc.CreateTable(auth, caller,
		`CREATE TABLE todoapp (
			complete BOOLEAN DEFAULT false,
			name     VARCHAR DEFAULT '',
			deleted  BOOLEAN DEFAULT false,
			id       SERIAL
		  );`)
	require.NoError(t, err)

	processCSV(t, caller.String(), tbld, "testdata/todoapp_queries.csv", backend)
}

func TestInsertOnConflict(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tbld, backend, sc, auth := setup(ctx, t)
	caller := common.HexToAddress("0xd43c59d5694ec111eb9e986c233200b14249558d")

	_, err := sc.CreateTable(auth, caller,
		`CREATE TABLE foo (
			name text unique,
			count int);`)
	require.NoError(t, err)
	backend.Commit()

	baseReq := tableland.RunSQLRequest{
		Controller: "0xd43c59d5694ec111eb9e986c233200b14249558d",
	}
	req := baseReq
	var txnHashes []string
	for i := 0; i < 10; i++ {
		req.Statement = `INSERT INTO _0 VALUES ('bar', 0) ON CONFLICT (name) DO UPDATE SET count=_0.count+1`
		r, err := tbld.RunSQL(ctx, req)
		require.NoError(t, err)
		backend.Commit()
		txnHashes = append(txnHashes, r.Transaction.Hash)
	}

	req.Statement = "SELECT count FROM _0"
	require.Eventually(
		t,
		jsonEq(t, tbld, req, `{"columns":[{"name":"count"}],"rows":[[9]]}`),
		time.Second*5,
		time.Millisecond*100,
	)
	requireReceipts(t, tbld, txnHashes, true)
}

func TestMultiStatement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tbld, backend, sc, auth := setup(ctx, t)
	caller := common.HexToAddress("0xd43c59d5694ec111eb9e986c233200b14249558d")

	_, err := sc.CreateTable(auth, caller,
		`CREATE TABLE foo (
			name text unique
		);`)
	require.NoError(t, err)

	req := tableland.RunSQLRequest{
		Controller: "0xd43c59d5694ec111eb9e986c233200b14249558d",
		Statement:  `INSERT INTO foo_0 values ('bar'); UPDATE foo_0 SET name='zoo'`,
	}
	r, err := tbld.RunSQL(ctx, req)
	require.NoError(t, err)
	backend.Commit()

	req.Statement = "SELECT name from _0"
	require.Eventually(
		t,
		jsonEq(t, tbld, req, `{"columns":[{"name":"name"}],"rows":[["zoo"]]}`),
		time.Second*5,
		time.Millisecond*100,
	)
	requireReceipts(t, tbld, []string{r.Transaction.Hash}, true)
}

func TestReadSystemTable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tbld, _, sc, auth := setup(ctx, t)
	caller := common.HexToAddress("0xd43c59d5694ec111eb9e986c233200b14249558d")

	_, err := sc.CreateTable(auth, caller, `CREATE TABLE foo (myjson JSON);`)
	require.NoError(t, err)

	res, err := runSQL(t, tbld, "select * from registry", "0xd43c59d5694ec111eb9e986c233200b14249558d")
	require.NoError(t, err)
	_, err = json.Marshal(res.Result)
	require.NoError(t, err)
}

func TestJSON(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tbld, backend, sc, auth := setup(ctx, t)
	caller := common.HexToAddress("0xd43c59d5694ec111eb9e986c233200b14249558d")

	_, err := sc.CreateTable(auth, caller, `CREATE TABLE foo (myjson JSON);`)
	require.NoError(t, err)

	processCSV(t, caller.Hex(), tbld, "testdata/json_queries.csv", backend)
}

func TestCheckInsertPrivileges(t *testing.T) {
	t.Parallel()
	granter := "0xd43c59d5694ec111eb9e986c233200b14249558d" // nolint
	grantee := "0x4afe8e30db4549384b0a05bb796468b130c7d6e0" // nolint

	ctx := context.Background()
	tbld, backend, sc, auth := setup(ctx, t)
	caller := common.HexToAddress("0xd43c59d5694ec111eb9e986c233200b14249558d")

	type testCase struct { // nolint
		query      string
		privileges tableland.Privileges
		isAllowed  bool
	}

	tests := []testCase{
		{"INSERT INTO foo_%s (bar) VALUES ('Hello')", tableland.Privileges{}, false},
		{"INSERT INTO foo_%s (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivInsert}, true},
		{"INSERT INTO foo_%s (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivUpdate}, false},
		{"INSERT INTO foo_%s (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivDelete}, false},
		{"INSERT INTO foo_%s (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate}, true},                       //nolint
		{"INSERT INTO foo_%s (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivInsert, tableland.PrivDelete}, true},                       //nolint
		{"INSERT INTO foo_%s (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivUpdate, tableland.PrivDelete}, false},                      //nolint
		{"INSERT INTO foo_%s (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate, tableland.PrivDelete}, true}, //nolint
	}

	for i, test := range tests {
		testCase := fmt.Sprint(i)
		t.Run(testCase, func(t *testing.T) {
			_, err := sc.CreateTable(auth, caller, `CREATE TABLE foo (bar text);`)
			require.NoError(t, err)
			backend.Commit()

			var successfulTxnHashes []string
			if len(test.privileges) > 0 {
				privileges := make([]string, len(test.privileges))
				for i, priv := range test.privileges {
					privileges[i] = priv.ToSQLString()
				}

				// execute grant statement according to test case
				grantQuery := fmt.Sprintf("GRANT %s ON foo_%s TO \"%s\"", strings.Join(privileges, ","), testCase, grantee)
				r, err := runSQL(t, tbld, grantQuery, granter)
				require.NoError(t, err)
				backend.Commit()
				successfulTxnHashes = append(successfulTxnHashes, r.Transaction.Hash)
			}

			r, err := runSQL(t, tbld, fmt.Sprintf(test.query, testCase), grantee)
			require.NoError(t, err)
			backend.Commit()

			testQuery := fmt.Sprintf("SELECT * FROM foo_%s WHERE bar ='Hello';", testCase)
			if test.isAllowed {
				require.Eventually(t, runSQLCountEq(t, tbld, testQuery, grantee, 1), 5*time.Second, 100*time.Millisecond)
				successfulTxnHashes = append(successfulTxnHashes, r.Transaction.Hash)
				requireReceipts(t, tbld, successfulTxnHashes, true)
			} else {
				require.Never(t, runSQLCountEq(t, tbld, testQuery, grantee, 1), 5*time.Second, 100*time.Millisecond)
				requireReceipts(t, tbld, successfulTxnHashes, true)
				requireReceipts(t, tbld, []string{r.Transaction.Hash}, false)
			}
		})
	}
}

func TestCheckUpdatePrivileges(t *testing.T) {
	t.Parallel()
	granter := "0xd43c59d5694ec111eb9e986c233200b14249558d"
	grantee := "0x4afe8e30db4549384b0a05bb796468b130c7d6e0"

	ctx := context.Background()
	tbld, backend, sc, auth := setup(ctx, t)
	caller := common.HexToAddress("0xd43c59d5694ec111eb9e986c233200b14249558d")

	type testCase struct { // nolint
		query      string
		privileges tableland.Privileges
		isAllowed  bool
	}

	tests := []testCase{
		{"UPDATE foo_%s SET bar = 'Hello 2'", tableland.Privileges{}, false},
		{"UPDATE foo_%s SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivInsert}, false},
		{"UPDATE foo_%s SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivUpdate}, true},
		{"UPDATE foo_%s SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivDelete}, false},
		{"UPDATE foo_%s SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate}, true},
		{"UPDATE foo_%s SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivInsert, tableland.PrivDelete}, false},
		{"UPDATE foo_%s SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivUpdate, tableland.PrivDelete}, true},
		{"UPDATE foo_%s SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate, tableland.PrivDelete}, true}, //nolint
	}

	for i, test := range tests {
		testCase := fmt.Sprint(i)
		t.Run(testCase, func(t *testing.T) {
			_, err := sc.CreateTable(auth, caller, `CREATE TABLE foo (bar text);`)
			require.NoError(t, err)
			backend.Commit()
			var successfulTxnHashes []string

			// we initilize the table with a row to be updated
			r, err := runSQL(t, tbld, fmt.Sprintf("INSERT INTO foo_%s (bar) VALUES ('Hello')", testCase), granter)
			require.NoError(t, err)
			backend.Commit()
			successfulTxnHashes = append(successfulTxnHashes, r.Transaction.Hash)

			if len(test.privileges) > 0 {
				privileges := make([]string, len(test.privileges))
				for i, priv := range test.privileges {
					privileges[i] = priv.ToSQLString()
				}

				// execute grant statement according to test case
				grantQuery := fmt.Sprintf("GRANT %s ON foo_%s TO \"%s\"", strings.Join(privileges, ","), testCase, grantee)
				r, err := runSQL(t, tbld, grantQuery, granter)
				require.NoError(t, err)
				backend.Commit()
				successfulTxnHashes = append(successfulTxnHashes, r.Transaction.Hash)
			}

			r, err = runSQL(t, tbld, fmt.Sprintf(test.query, testCase), grantee)
			require.NoError(t, err)
			backend.Commit()

			testQuery := fmt.Sprintf("SELECT * FROM foo_%s WHERE bar ='Hello 2';", testCase)
			if test.isAllowed {
				require.Eventually(t, runSQLCountEq(t, tbld, testQuery, grantee, 1), 5*time.Second, 100*time.Millisecond)
				successfulTxnHashes = append(successfulTxnHashes, r.Transaction.Hash)
				requireReceipts(t, tbld, successfulTxnHashes, true)
			} else {
				require.Never(t, runSQLCountEq(t, tbld, testQuery, grantee, 1), 5*time.Second, 100*time.Millisecond)
				requireReceipts(t, tbld, successfulTxnHashes, true)
				requireReceipts(t, tbld, []string{r.Transaction.Hash}, false)
			}
		})
	}
}

func TestCheckDeletePrivileges(t *testing.T) {
	t.Parallel()
	granter := "0xd43c59d5694ec111eb9e986c233200b14249558d"
	grantee := "0x4afe8e30db4549384b0a05bb796468b130c7d6e0"

	ctx := context.Background()
	tbld, backend, sc, auth := setup(ctx, t)
	caller := common.HexToAddress("0xd43c59d5694ec111eb9e986c233200b14249558d")

	type testCase struct { // nolint
		query      string
		privileges tableland.Privileges
		isAllowed  bool
	}

	tests := []testCase{
		{"DELETE FROM foo_%s", tableland.Privileges{}, false},
		{"DELETE FROM foo_%s", tableland.Privileges{tableland.PrivInsert}, false},
		{"DELETE FROM foo_%s", tableland.Privileges{tableland.PrivUpdate}, false},
		{"DELETE FROM foo_%s", tableland.Privileges{tableland.PrivDelete}, true},
		{"DELETE FROM foo_%s", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate}, false},
		{"DELETE FROM foo_%s", tableland.Privileges{tableland.PrivInsert, tableland.PrivDelete}, true},
		{"DELETE FROM foo_%s", tableland.Privileges{tableland.PrivUpdate, tableland.PrivDelete}, true},
		{"DELETE FROM foo_%s", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate, tableland.PrivDelete}, true}, //nolint
	}

	for i, test := range tests {
		testCase := fmt.Sprint(i)
		t.Run(testCase, func(t *testing.T) {
			_, err := sc.CreateTable(auth, caller, `CREATE TABLE foo (bar text);`)
			require.NoError(t, err)
			var successfulTxnHashes []string

			// we initilize the table with a row to be delete
			_, err = runSQL(t, tbld, fmt.Sprintf("INSERT INTO foo_%s (bar) VALUES ('Hello')", testCase), granter)
			require.NoError(t, err)
			backend.Commit()

			if len(test.privileges) > 0 {
				privileges := make([]string, len(test.privileges))
				for i, priv := range test.privileges {
					privileges[i] = priv.ToSQLString()
				}

				// execute grant statement according to test case
				grantQuery := fmt.Sprintf("GRANT %s ON foo_%s TO \"%s\"", strings.Join(privileges, ","), testCase, grantee)
				r, err := runSQL(t, tbld, grantQuery, granter)
				require.NoError(t, err)
				backend.Commit()
				successfulTxnHashes = append(successfulTxnHashes, r.Transaction.Hash)
			}

			r, err := runSQL(t, tbld, fmt.Sprintf(test.query, testCase), grantee)
			require.NoError(t, err)
			backend.Commit()

			testQuery := fmt.Sprintf("SELECT * FROM foo_%s", testCase)
			if test.isAllowed {
				require.Eventually(t, runSQLCountEq(t, tbld, testQuery, grantee, 0), 5*time.Second, 100*time.Millisecond)
				successfulTxnHashes = append(successfulTxnHashes, r.Transaction.Hash)
				requireReceipts(t, tbld, successfulTxnHashes, true)
			} else {
				require.Never(t, runSQLCountEq(t, tbld, testQuery, grantee, 0), 5*time.Second, 100*time.Millisecond)
				requireReceipts(t, tbld, []string{r.Transaction.Hash}, false)
			}
		})
	}
}

func TestOwnerRevokesItsPrivilegeInsideMultipleStatements(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tbld, backend, sc, auth := setup(ctx, t)
	caller := common.HexToAddress("0xd43c59d5694ec111eb9e986c233200b14249558d")

	_, err := sc.CreateTable(auth, caller, `CREATE TABLE foo (bar text);`)
	require.NoError(t, err)

	multiStatements := `
		INSERT INTO foo_0 (bar) VALUES ('Hello');
		UPDATE foo_0 SET bar = 'Hello 2';
		REVOKE update ON foo_0 FROM "0xd43c59d5694ec111eb9e986c233200b14249558d";
		UPDATE foo_0 SET bar = 'Hello 3';
	`
	r, err := runSQL(t, tbld, multiStatements, "0xd43c59d5694ec111eb9e986c233200b14249558d")
	require.NoError(t, err)
	backend.Commit()

	testQuery := "SELECT * FROM foo_0;"
	cond := runSQLCountEq(t, tbld, testQuery, "0xd43c59d5694ec111eb9e986c233200b14249558d", 1)
	require.Never(t, cond, 5*time.Second, 100*time.Millisecond)
	requireReceipts(t, tbld, []string{r.Transaction.Hash}, false)
}

func processCSV(
	t *testing.T,
	controller string,
	tbld tableland.Tableland,
	csvPath string,
	backend *backends.SimulatedBackend) {
	t.Helper()

	baseReq := tableland.RunSQLRequest{
		Controller: controller,
	}
	records := readCsvFile(t, csvPath)
	for _, record := range records {
		req := baseReq
		req.Statement = record[1]

		if record[0] == "r" {
			require.Eventually(t, jsonEq(t, tbld, req, record[2]), time.Second*5, time.Millisecond*100)
		} else {
			_, err := tbld.RunSQL(context.Background(), req)
			require.NoError(t, err)
			backend.Commit()
		}
	}
}

func jsonEq(t *testing.T, tbld tableland.Tableland, req tableland.RunSQLRequest, expJSON string) func() bool {
	return func() bool {
		r, err := tbld.RunSQL(context.Background(), req)
		require.NoError(t, err)

		b, err := json.Marshal(r.Result)
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

func runSQLCountEq(t *testing.T, tbld tableland.Tableland, sql string, address string, expCount int) func() bool {
	return func() bool {
		response, err := runSQL(t, tbld, sql, address)
		require.NoError(t, err)

		responseInBytes, err := json.Marshal(response)
		require.NoError(t, err)

		r := &struct {
			Data struct {
				Rows [][]interface{} `json:"rows"`
			} `json:"data"`
		}{}

		err = json.Unmarshal(responseInBytes, r)
		require.NoError(t, err)

		return len(r.Data.Rows) == expCount
	}
}

func runSQL(t *testing.T, tbld tableland.Tableland, sql string, controller string) (tableland.RunSQLResponse, error) {
	t.Helper()

	req := tableland.RunSQLRequest{
		Controller: controller,
		Statement:  sql,
	}

	return tbld.RunSQL(context.Background(), req)
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

func setup(
	ctx context.Context,
	t *testing.T) (tableland.Tableland,
	*backends.SimulatedBackend,
	*ethereum.Contract,
	*bind.TransactOpts) {
	t.Helper()

	url := tests.PostgresURL(t)

	sqlstore, err := sqlstoreimpl.New(ctx, url)
	require.NoError(t, err)

	parser := parserimpl.New([]string{"system_", "registry"}, 0, 0)
	txnp, err := txnpimpl.NewTxnProcessor(url, 0, &aclHalfMock{sqlstore})
	require.NoError(t, err)

	backend, addr, sc, auth, sk := testutil.Setup(t)

	wallet, err := wallet.NewWallet(sk)
	require.NoError(t, err)

	registry, err := ethereum.NewClient(
		backend,
		1337,
		addr,
		wallet,
		impl.NewSimpleTracker(wallet, backend),
	)
	require.NoError(t, err)
	tbld := NewTablelandMesa(sqlstore, parser, txnp, &aclHalfMock{sqlstore}, registry, 1337)

	// Spin up dependencies needed for the EventProcessor.
	// i.e: TxnProcessor, Parser, and EventFeed (connected to the EVM chain)
	ef, err := efimpl.New(backend, addr, eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)

	// Create EventProcessor for our test.
	ep, err := epimpl.New(parser, txnp, ef, 1337)
	require.NoError(t, err)

	err = ep.Start()
	require.NoError(t, err)
	t.Cleanup(func() { ep.Stop() })

	return tbld, backend, sc, auth
}

type aclHalfMock struct {
	sqlStore sqlstore.SQLStore
}

func (acl *aclHalfMock) CheckPrivileges(
	ctx context.Context,
	tx pgx.Tx,
	controller common.Address,
	id tableland.TableID,
	op tableland.Operation) (bool, error) {
	aclImpl := NewACL(acl.sqlStore, nil)
	return aclImpl.CheckPrivileges(ctx, tx, controller, id, op)
}

func (acl *aclHalfMock) CheckAuthorization(ctx context.Context, controller common.Address) error {
	return nil
}

func (acl *aclHalfMock) IsOwner(ctx context.Context, controller common.Address, id tableland.TableID) (bool, error) {
	return true, nil
}

func requireReceipts(t *testing.T, tbld tableland.Tableland, txnHashes []string, ok bool) {
	t.Helper()

	for _, txnHash := range txnHashes {
		r, err := tbld.GetReceipt(context.Background(), tableland.GetReceiptRequest{
			TxnHash: txnHash,
		})
		require.NoError(t, err)
		require.True(t, r.Ok)
		require.NotNil(t, r.Receipt)
		require.Equal(t, int64(1337), r.Receipt.ChainID)
		require.Equal(t, txnHash, txnHash)
		require.NotZero(t, r.Receipt.BlockNumber)
		if ok {
			require.Nil(t, r.Receipt.Error)
			require.NotNil(t, r.Receipt.TableID)
			require.NotZero(t, r.Receipt.TableID)
		} else {
			require.NotNil(t, r.Receipt.Error)
			require.NotEmpty(t, *r.Receipt.Error)
			require.Nil(t, r.Receipt.TableID)
		}
	}
}
