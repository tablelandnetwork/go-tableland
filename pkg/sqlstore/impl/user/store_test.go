package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestReadGeneralTypeCorrectness(t *testing.T) {
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	require.NoError(t, err)

	ctx := context.Background()
	db.Begin()
	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	{
		data, err := execReadQuery(ctx, tx, "SELECT 1 one")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"one"}],"rows":[[1]]}`, string(b))
	}

	{
		data, err := execReadQuery(ctx, tx, "SELECT 1 a, 2 b")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"a"}, {"name":"b"}],"rows":[[1, 2]]}`, string(b))
	}

	// test real type parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT cast(1.2 as real) float8")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"float8"}],"rows":[[1.2]]}`, string(b))
	}

	// test text type parsing
	{
		data, err := execReadQuery(ctx, tx, "SELECT 'hello' text")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"text"}],"rows":[["hello"]]}`, string(b))
	}

	/* TODO(jsign)
	// test json map
	{
		data, err := execReadQuery(ctx, tx, `SELECT '{"foo": 1, "bar":"zar"}' json;`)
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
	*/
}
