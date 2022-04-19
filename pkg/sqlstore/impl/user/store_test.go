package user

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/tests"
)

func TestReadGeneralTypeCorrectness(t *testing.T) {
	url := tests.PostgresURL(t)

	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	require.NoError(t, err)
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	{
		data, err := execReadQuery(ctx, tx, "SELECT 1")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"?column?"}],"rows":[[1]]}`, string(b))
	}

	{
		data, err := execReadQuery(ctx, tx, "SELECT 1 a, 2 b")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"a"}, {"name":"b"}],"rows":[[1, 2]]}`, string(b))
	}

	// test float type parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT 1.2::float")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"float8"}],"rows":[[1.2]]}`, string(b))
	}

	// test decimal type parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT 123.456789::decimal")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"numeric"}],"rows":[["123.456789"]]}`, string(b))
	}

	// test numeric type float parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT 123.456789::numeric")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"numeric"}],"rows":[["123.456789"]]}`, string(b))
	}

	// test numeric type int parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT 100::numeric")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"numeric"}],"rows":[["100"]]}`, string(b))
	}

	// test bool type parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT true::bool, false::bool")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"bool"}, {"name":"bool"}],"rows":[[true, false]]}`, string(b))
	}

	// test text type parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT 'hello'::text")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"text"}],"rows":[["hello"]]}`, string(b))
	}
	// test varchar type parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT '2014-01-01'::varchar")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"varchar"}],"rows":[["2014-01-01"]]}`, string(b))
	}

	// test date type parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT '2014-01-01'::date")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"date"}],"rows":[["2014-01-01"]]}`, string(b))
	}

	// test timestamp type parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT '2014-01-01'::timestamp")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"timestamp"}],"rows":[["2014-01-01 00:00:00"]]}`, string(b))
	}

	// test timestamptz type parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT '2014-01-01'::timestamptz")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"timestamptz"}],"rows":[["2014-01-01 00:00:00Z"]]}`, string(b))
	}

	// test uuid type parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT '00000000-0000-0000-0000-000000000000'::uuid;")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"uuid"}],"rows":[["00000000-0000-0000-0000-000000000000"]]}`, string(b))
	}
	// test json null type parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT (null)::json;")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"json"}],"rows":[[null]]}`, string(b))
	}
	// test json map
	{
		data, err := execReadQuery(ctx, tx, `SELECT ('{"foo": 1, "bar":"zar"}')::json;`)
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"json"}],"rows":[[{"bar":"zar","foo":1}]]}`, string(b))
	}
	// test json array
	{
		data, err := execReadQuery(ctx, tx, `SELECT ('[{"foo": 1}, {"bar":[1,2]}]')::json;`)
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"json"}],"rows":[[[{"foo":1},{"bar":[1,2]}]]]}`, string(b))
	}
	// test json single string
	{
		data, err := execReadQuery(ctx, tx, `SELECT ('"iam valid too"')::json;`)
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"json"}],"rows":[["iam valid too"]]}`, string(b))
	}
}
