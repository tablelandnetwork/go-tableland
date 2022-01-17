package user

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/tests"
)

func TestReadGeneralTypeCorrectness(t *testing.T) {
	url, err := tests.PostgresURL()
	require.NoError(t, err)

	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	require.NoError(t, err)
	defer pool.Close()
	userStore := New(pool)

	{
		data, err := userStore.Read(ctx, "SELECT 1")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"?column?"}],"rows":[[1]]}`, string(b))
	}

	{
		data, err := userStore.Read(ctx, "SELECT 1 a, 2 b")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"a"}, {"name":"b"}],"rows":[[1, 2]]}`, string(b))
	}

	// test float type parsing
	{
		data, err := userStore.Read(ctx, "SELECT 1.2::float")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"float8"}],"rows":[[1.2]]}`, string(b))
	}

	// test decimal type parsing
	{
		data, err := userStore.Read(ctx, "SELECT 1.2::decimal")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"numeric"}],"rows":[["12e-1"]]}`, string(b))
	}

	// test numeric type parsing
	{
		data, err := userStore.Read(ctx, "SELECT 1.2::numeric")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"numeric"}],"rows":[["12e-1"]]}`, string(b))
	}

	// test bool type parsing
	{
		data, err := userStore.Read(ctx, "SELECT true::bool, false::bool")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"bool"}, {"name":"bool"}],"rows":[[true, false]]}`, string(b))
	}

	// test text type parsing
	{
		data, err := userStore.Read(ctx, "SELECT 'hello'::text")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"text"}],"rows":[["hello"]]}`, string(b))
	}

	// test varchar type parsing
	{
		data, err := userStore.Read(ctx, "SELECT '2014-01-01'::varchar")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"varchar"}],"rows":[["2014-01-01"]]}`, string(b))
	}

	// test date type parsing
	{
		data, err := userStore.Read(ctx, "SELECT '2014-01-01'::date")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"date"}],"rows":[["2014-01-01"]]}`, string(b))
	}

	// test timestamp type parsing
	{
		data, err := userStore.Read(ctx, "SELECT '2014-01-01'::timestamp")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"timestamp"}],"rows":[["2014-01-01 00:00:00"]]}`, string(b))
	}

	// test timestamptz type parsing
	{
		data, err := userStore.Read(ctx, "SELECT '2014-01-01'::timestamptz")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"timestamptz"}],"rows":[["2014-01-01 00:00:00Z"]]}`, string(b))
	}

	// test uuid type parsing
	{
		data, err := userStore.Read(ctx, "SELECT '00000000-0000-0000-0000-000000000000'::uuid;")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"uuid"}],"rows":[["00000000-0000-0000-0000-000000000000"]]}`, string(b))
	}
}

func TestTodoAppWorkflow(t *testing.T) {
	url, err := tests.PostgresURL()
	require.NoError(t, err)

	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	require.NoError(t, err)

	userStore := New(pool)

	// creates todo app table
	{
		err := userStore.Write(ctx, `CREATE TABLE todoapp (
			complete BOOLEAN DEFAULT false,
			name     VARCHAR DEFAULT '',
			deleted  BOOLEAN DEFAULT false,
			id       SERIAL
		  );`)

		require.NoError(t, err)
	}

	records := readCsvFile("testdata/todoapp_queries.csv")
	for _, record := range records {
		if record[0] == "w" {
			err := userStore.Write(ctx, record[1])
			require.NoError(t, err)
		} else if record[0] == "r" {
			data, err := userStore.Read(ctx, record[1])
			require.NoError(t, err)
			b, err := json.Marshal(data)
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
