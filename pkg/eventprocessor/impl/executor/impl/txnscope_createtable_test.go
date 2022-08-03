package impl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
)

func TestRegisterTable(t *testing.T) {
	t.Parallel()

	parser := newParser(t, []string{})
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txnp, dbURL := newExecutor(t, 0)

		b, err := txnp.OpenBatch(ctx)
		require.NoError(t, err)

		id, err := tableland.NewTableID("100")
		require.NoError(t, err)
		createStmt, err := parser.ValidateCreateTable("create table bar_1337 (zar text)", 1337)
		require.NoError(t, err)
		err = b.InsertTable(ctx, id, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", createStmt)
		require.NoError(t, err)

		require.NoError(t, b.Commit())
		require.NoError(t, b.Close())
		require.NoError(t, txnp.Close(ctx))

		// Check that the table was registered in the system-table.
		systemStore, err := system.New(dbURL, tableland.ChainID(chainID))
		require.NoError(t, err)
		table, err := systemStore.GetTable(ctx, id)
		require.NoError(t, err)
		require.Equal(t, id, table.ID)
		require.Equal(t, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", table.Controller)
		// echo -n zar:TEXT | shasum -a 256
		require.Equal(t, "7ec5320c16e06e90af5e7131ff0c80d4b0a08fcd62aa6e38ad8d6843bc480d09", table.Structure)
		require.Equal(t, "bar", table.Prefix)
		require.NotEqual(t, new(time.Time), table.CreatedAt) // CreatedAt is not the zero value

		// Check that the user table was created.
		ok := existsTableWithName(t, dbURL, "bar_1337_100")
		require.True(t, ok)
	})
}
