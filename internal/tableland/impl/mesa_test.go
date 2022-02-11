package impl

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"log"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
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
		Controller:  "ctrl-1",
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
			Controller:  "ctrl-1",
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
			Controller: "ctrl-1",
		}
		req := baseReq
		for i := 0; i < 10; i++ {
			req.Statement = `INSERT INTO _1337 VALUES ('bar', 0) ON CONFLICT (name) DO UPDATE SET count=_1337.count+1`
			_, err := tbld.RunSQL(ctx, req)
			if err != nil {
				require.NoError(t, err)
			}
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
			Controller:  "ctrl-1",
			Statement: `CREATE TABLE foo (
			name text unique
		);`,
		}
		_, err := tbld.CreateTable(ctx, req)
		require.NoError(t, err)
	}

	{
		req := tableland.RunSQLRequest{
			Controller: "ctrl-1",
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
		Controller:  "ctrl-1",
		Statement:   `CREATE TABLE foo (myjson JSON);`,
	}
	_, err := tbld.CreateTable(ctx, req)
	require.NoError(t, err)

	req2 := tableland.RunSQLRequest{
		Controller: "ctrl-1",
		Statement:  `select * from system_tables`,
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
		Controller:  "ctrl-1",
		Statement:   `CREATE TABLE foo (myjson JSON);`,
	}
	_, err := tbld.CreateTable(ctx, req)
	require.NoError(t, err)

	processCSV(t, req.Controller, tbld, "testdata/json_queries.csv")
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
	err = sqlstore.Authorize(ctx, "ctrl-1")
	require.NoError(t, err)
	parser := parserimpl.New("system_", 0, 0)
	txnp, err := txnpimpl.NewTxnProcessor(url, 0)
	require.NoError(t, err)

	return NewTablelandMesa(sqlstore, &dummyRegistry{}, parser, txnp)
}

type dummyRegistry struct{}

func (dr *dummyRegistry) IsOwner(context context.Context, addrress common.Address, id *big.Int) (bool, error) {
	return true, nil
}
