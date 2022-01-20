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
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/tests"
)

func TestTodoAppWorkflow(t *testing.T) {
	url, err := tests.PostgresURL()
	require.NoError(t, err)
	ctx := context.Background()

	sqlstore, err := sqlstoreimpl.New(ctx, url)
	require.NoError(t, err)
	parser := parserimpl.New("system_")
	tbld := NewTablelandMesa(sqlstore, &dummyRegistry{}, parser)

	baseReq := tableland.Request{
		TableID:    uuid.New().String(),
		Type:       "type-1",
		Controller: "ctrl-1",
	}
	// creates todo app table
	{
		req := baseReq
		req.Statement = `CREATE TABLE todoapp (
			complete BOOLEAN DEFAULT false,
			name     VARCHAR DEFAULT '',
			deleted  BOOLEAN DEFAULT false,
			id       SERIAL
		  );`
		_, err := tbld.CreateTable(ctx, req)
		require.NoError(t, err)
	}

	processCSV(t, baseReq, tbld, "testdata/todoapp_queries.csv")
}

func TestJSONB(t *testing.T) {
	url, err := tests.PostgresURL()
	require.NoError(t, err)
	ctx := context.Background()

	sqlstore, err := sqlstoreimpl.New(ctx, url)
	require.NoError(t, err)
	parser := parserimpl.New("system_")
	tbld := NewTablelandMesa(sqlstore, &dummyRegistry{}, parser)

	baseReq := tableland.Request{
		TableID:    uuid.New().String(),
		Type:       "type-1",
		Controller: "ctrl-1",
	}

	{
		req := baseReq
		req.Statement = `CREATE TABLE foo (myjson JSONB);`
		_, err := tbld.CreateTable(ctx, req)
		require.NoError(t, err)
	}

	processCSV(t, baseReq, tbld, "testdata/json_queries.csv")
}

func processCSV(t *testing.T, baseReq tableland.Request, tbld tableland.Tableland, csvPath string) {
	t.Helper()
	records := readCsvFile(csvPath)
	for _, record := range records {
		req := baseReq
		req.Statement = record[1]
		r, err := tbld.RunSQL(context.Background(), req)
		require.NoError(t, err)

		if record[0] == "r" {
			b, err := json.Marshal(r.Data)
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

type dummyRegistry struct{}

func (dr *dummyRegistry) IsOwner(context context.Context, addrress common.Address, id *big.Int) (bool, error) {
	return true, nil
}
