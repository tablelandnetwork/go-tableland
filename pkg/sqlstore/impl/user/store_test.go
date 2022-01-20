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
	// test jsonb null type parsing
	{
		data, err := userStore.Read(ctx, "SELECT (null)::jsonb;")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"jsonb"}],"rows":[[null]]}`, string(b))
	}
}
