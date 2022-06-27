package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/tests"
)

func TestReadGeneralTypeCorrectness(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite3", tests.Sqlite3URL())
	require.NoError(t, err)

	ctx := context.Background()

	// INTEGER
	{
		data, err := execReadQuery(ctx, db, "SELECT cast(1 as INTEGER) one")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"one"}],"rows":[[1]]}`, string(b))
	}

	// Two INTEGERs without cast.
	{
		data, err := execReadQuery(ctx, db, "SELECT 1 a, 2 b")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"a"}, {"name":"b"}],"rows":[[1, 2]]}`, string(b))
	}

	// REAL
	{
		data, err := execReadQuery(ctx, db, "SELECT cast(1.2 as REAL) real")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"real"}],"rows":[[1.2]]}`, string(b))
	}

	// TEXT
	{
		data, err := execReadQuery(ctx, db, "SELECT 'hello' text")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"text"}],"rows":[["hello"]]}`, string(b))
	}

	// BLOB
	{
		data, err := execReadQuery(ctx, db, "SELECT cast(X'4141414141414141414141' as BLOB) blob")
		require.NoError(t, err)
		b, err := json.Marshal(data)
		require.NoError(t, err)
		require.JSONEq(t, `{"columns":[{"name":"blob"}],"rows":[["QUFBQUFBQUFBQUE="]]}`, string(b))
	}
}
