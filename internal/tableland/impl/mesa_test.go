package impl

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	txnpimpl "github.com/textileio/go-tableland/pkg/txn/impl"
	"github.com/textileio/go-tableland/tests"
)

func TestTodoAppWorkflow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tbld := newTablelandMesa(t)

	req := tableland.CreateTableRequest{
		ID:          "1337",
		Description: "descrip-1",
		Controller:  "0xd43c59d5694ec111eb9e986c233200b14249558d",
		Statement: `CREATE TABLE todoapp (
			complete BOOLEAN DEFAULT false,
			name     VARCHAR DEFAULT '',
			deleted  BOOLEAN DEFAULT false,
			id       SERIAL
		  );`,
	}
	_, err := tbld.CreateTable(ctx, req)
	require.NoError(t, err)

	processCSV(t, req.Controller, tbld, "testdata/todoapp_queries.csv")
}

func TestInsertOnConflict(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tbld := newTablelandMesa(t)

	{
		req := tableland.CreateTableRequest{
			ID:          "1337",
			Description: "descrip-1",
			Controller:  "0xd43c59d5694ec111eb9e986c233200b14249558d",
			Statement: `CREATE TABLE foo (
			name text unique,
			count int 
		);`,
		}
		_, err := tbld.CreateTable(ctx, req)
		require.NoError(t, err)
	}

	{
		baseReq := tableland.RunSQLRequest{
			Controller: "0xd43c59d5694ec111eb9e986c233200b14249558d",
		}
		req := baseReq
		for i := 0; i < 10; i++ {
			req.Statement = `INSERT INTO _1337 VALUES ('bar', 0) ON CONFLICT (name) DO UPDATE SET count=_1337.count+1`
			_, err := tbld.RunSQL(ctx, req)
			require.NoError(t, err)
		}

		req.Statement = "SELECT count FROM _1337"
		res, err := tbld.RunSQL(ctx, req)
		require.NoError(t, err)
		js, err := json.Marshal(res.Result)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"count"}],"rows":[[9]]}`, string(js))
	}
}

func TestMultiStatement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tbld := newTablelandMesa(t)

	{
		req := tableland.CreateTableRequest{
			ID:          "1",
			Description: "descrp-1",
			Controller:  "0xd43c59d5694ec111eb9e986c233200b14249558d",
			Statement: `CREATE TABLE foo (
			name text unique
		);`,
		}
		_, err := tbld.CreateTable(ctx, req)
		require.NoError(t, err)
	}

	{
		req := tableland.RunSQLRequest{
			Controller: "0xd43c59d5694ec111eb9e986c233200b14249558d",
			Statement:  `INSERT INTO foo_1 values ('bar'); UPDATE foo_1 SET name='zoo'`,
		}
		_, err := tbld.RunSQL(ctx, req)
		require.NoError(t, err)

		req.Statement = "SELECT name from _1"
		res, err := tbld.RunSQL(ctx, req)
		require.NoError(t, err)
		js, err := json.Marshal(res.Result)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"name"}],"rows":[["zoo"]]}`, string(js))
	}
}

func TestReadSystemTable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tbld := newTablelandMesa(t)

	req := tableland.CreateTableRequest{
		ID:          "1337",
		Description: "descrp-1",
		Controller:  "0xd43c59d5694ec111eb9e986c233200b14249558d",
		Statement:   `CREATE TABLE foo (myjson JSON);`,
	}
	_, err := tbld.CreateTable(ctx, req)
	require.NoError(t, err)

	req2 := tableland.RunSQLRequest{
		Controller: "0xd43c59d5694ec111eb9e986c233200b14249558d",
		Statement:  `select * from registry`,
	}
	res, err := tbld.RunSQL(ctx, req2)
	require.NoError(t, err)
	_, err = json.Marshal(res.Result)
	require.NoError(t, err)
}

func TestJSON(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tbld := newTablelandMesa(t)

	req := tableland.CreateTableRequest{
		ID:          "1337",
		Description: "descrp-1",
		Controller:  "0xd43c59d5694ec111eb9e986c233200b14249558d",
		Statement:   `CREATE TABLE foo (myjson JSON);`,
	}
	_, err := tbld.CreateTable(ctx, req)
	require.NoError(t, err)

	processCSV(t, req.Controller, tbld, "testdata/json_queries.csv")
}

func TestCheckPrivileges(t *testing.T) {
	granter := "0xd43c59d5694ec111eb9e986c233200b14249558d"
	grantee := "0x4afe8e30db4549384b0a05bb796468b130c7d6e0"

	t.Parallel()

	type testCase struct {
		query      string
		privileges tableland.Privileges
		isAllowed  bool
	}

	tests := []testCase{
		{"INSERT INTO foo_1337 (bar) VALUES ('Hello')", tableland.Privileges{}, false},
		{"INSERT INTO foo_1337 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivInsert}, true},
		{"INSERT INTO foo_1337 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivUpdate}, false},
		{"INSERT INTO foo_1337 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivDelete}, false},
		{"INSERT INTO foo_1337 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate}, true},                       //nolint
		{"INSERT INTO foo_1337 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivInsert, tableland.PrivDelete}, true},                       //nolint
		{"INSERT INTO foo_1337 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivUpdate, tableland.PrivDelete}, false},                      //nolint
		{"INSERT INTO foo_1337 (bar) VALUES ('Hello')", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate, tableland.PrivDelete}, true}, //nolint

		{"UPDATE foo_1337 SET bar = 'Hello 2'", tableland.Privileges{}, false},
		{"UPDATE foo_1337 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivInsert}, false},
		{"UPDATE foo_1337 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivUpdate}, true},
		{"UPDATE foo_1337 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivDelete}, false},
		{"UPDATE foo_1337 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate}, true},
		{"UPDATE foo_1337 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivInsert, tableland.PrivDelete}, false},
		{"UPDATE foo_1337 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivUpdate, tableland.PrivDelete}, true},
		{"UPDATE foo_1337 SET bar = 'Hello 2'", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate, tableland.PrivDelete}, true}, //nolint

		{"DELETE FROM foo_1337", tableland.Privileges{}, false},
		{"DELETE FROM foo_1337", tableland.Privileges{tableland.PrivInsert}, false},
		{"DELETE FROM foo_1337", tableland.Privileges{tableland.PrivUpdate}, false},
		{"DELETE FROM foo_1337", tableland.Privileges{tableland.PrivDelete}, true},
		{"DELETE FROM foo_1337", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate}, false},
		{"DELETE FROM foo_1337", tableland.Privileges{tableland.PrivInsert, tableland.PrivDelete}, true},
		{"DELETE FROM foo_1337", tableland.Privileges{tableland.PrivUpdate, tableland.PrivDelete}, true},
		{"DELETE FROM foo_1337", tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate, tableland.PrivDelete}, true}, //nolint
	}

	ctx := context.Background()
	tbld := newTablelandMesa(t)

	req := tableland.CreateTableRequest{
		ID:          "1337",
		Description: "descrp-1",
		Controller:  granter,
		Statement:   `CREATE TABLE foo (bar text);`,
	}
	_, err := tbld.CreateTable(ctx, req)
	require.NoError(t, err)

	for i, test := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			// reset privileges
			revokeReq := tableland.RunSQLRequest{
				Controller: granter,
				Statement:  fmt.Sprintf("REVOKE insert, update, delete ON foo_1337 FROM \"%s\"", grantee),
			}
			_, err = tbld.RunSQL(ctx, revokeReq)
			require.NoError(t, err)

			if len(test.privileges) > 0 {
				privileges := make([]string, len(test.privileges))
				for i, priv := range test.privileges {
					privileges[i] = priv.ToSQLString()
				}

				// execute grant statement according to test case
				grantReq := tableland.RunSQLRequest{
					Controller: granter,
					Statement:  fmt.Sprintf("GRANT %s ON foo_1337 TO \"%s\"", strings.Join(privileges, ","), grantee),
				}
				_, err := tbld.RunSQL(ctx, grantReq)
				require.NoError(t, err)
			}

			req := tableland.RunSQLRequest{
				Controller: grantee,
				Statement:  test.query,
			}
			_, err := tbld.RunSQL(ctx, req)
			require.Equal(t, test.isAllowed, err == nil)
		})
	}

	// now we do the reverse. gives all privileges, and revokes one by one
	for i, test := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			// gives all privileges
			grantReq := tableland.RunSQLRequest{
				Controller: granter,
				Statement:  fmt.Sprintf("GRANT insert, update, delete ON foo_1337 TO \"%s\"", grantee),
			}
			_, err = tbld.RunSQL(ctx, grantReq)
			require.NoError(t, err)

			if len(test.privileges) > 0 {
				privileges := make([]string, len(test.privileges))
				for i, priv := range test.privileges {
					privileges[i] = priv.ToSQLString()
				}

				// execute revoke statement according to test case
				grantReq := tableland.RunSQLRequest{
					Controller: granter,
					Statement:  fmt.Sprintf("REVOKE %s ON foo_1337 FROM \"%s\"", strings.Join(privileges, ","), grantee),
				}
				_, err := tbld.RunSQL(ctx, grantReq)
				require.NoError(t, err)
			}

			req := tableland.RunSQLRequest{
				Controller: grantee,
				Statement:  test.query,
			}
			_, err := tbld.RunSQL(ctx, req)
			require.Equal(t, !test.isAllowed, err == nil)
		})
	}
}

func TestOwnerRevokesItsPrivilegeInsideMultipleStatements(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tbld := newTablelandMesa(t)

	req := tableland.CreateTableRequest{
		ID:          "1337",
		Description: "descrp-1",
		Controller:  "0xd43c59d5694ec111eb9e986c233200b14249558d",
		Statement:   `CREATE TABLE foo (bar text);`,
	}
	_, err := tbld.CreateTable(ctx, req)
	require.NoError(t, err)

	multiStatements := `
		INSERT INTO foo_1337 (bar) VALUES ('Hello');
		UPDATE foo_1337 SET bar = 'Hello 2';
		REVOKE update ON foo_1337 FROM "0xd43c59d5694ec111eb9e986c233200b14249558d";
		UPDATE foo_1337 SET bar = 'Hello 3';
	`

	grantReq := tableland.RunSQLRequest{
		Controller: "0xd43c59d5694ec111eb9e986c233200b14249558d",
		Statement:  multiStatements,
	}
	_, err = tbld.RunSQL(ctx, grantReq)

	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot execute operation, missing privilege=update")
}

func processCSV(t *testing.T, controller string, tbld tableland.Tableland, csvPath string) {
	t.Helper()
	baseReq := tableland.RunSQLRequest{
		Controller: controller,
	}
	records := readCsvFile(csvPath)
	for _, record := range records {
		req := baseReq
		req.Statement = record[1]
		r, err := tbld.RunSQL(context.Background(), req)
		require.NoError(t, err)

		if record[0] == "r" {
			b, err := json.Marshal(r.Result)
			require.NoError(t, err)
			require.JSONEq(t, record[2], string(b))
		}
	}
}

func readCsvFile(filePath string) [][]string {
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatal("Unable to read input file "+filePath, err)
	}
	defer f.Close() // nolint

	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.Fatal("Unable to parse file as CSV for "+filePath, err)
	}

	return records
}

func newTablelandMesa(t *testing.T) tableland.Tableland {
	t.Helper()
	url, err := tests.PostgresURL()
	require.NoError(t, err)

	ctx := context.Background()
	sqlstore, err := sqlstoreimpl.New(ctx, url)
	require.NoError(t, err)
	err = sqlstore.Authorize(ctx, "0xd43c59d5694ec111eb9e986c233200b14249558d")
	require.NoError(t, err)
	parser := parserimpl.New([]string{"system_", "registry"}, 0, 0)
	txnp, err := txnpimpl.NewTxnProcessor(url, 0, &aclHalfMock{sqlstore})
	require.NoError(t, err)

	return NewTablelandMesa(sqlstore, parser, txnp, &aclHalfMock{sqlstore})
}

type aclHalfMock struct {
	sqlStore sqlstore.SQLStore
}

func (acl *aclHalfMock) CheckPrivileges(
	ctx context.Context,
	tx pgx.Tx,
	controller common.Address,
	id tableland.TableID,
	op tableland.Operation) error {
	aclImpl := NewACL(acl.sqlStore, nil)
	return aclImpl.CheckPrivileges(ctx, tx, controller, id, op)
}

func (acl *aclHalfMock) CheckAuthorization(ctx context.Context, controller common.Address) error {
	return nil
}

func (acl *aclHalfMock) IsOwner(ctx context.Context, controller common.Address, id tableland.TableID) (bool, error) {
	return true, nil
}
