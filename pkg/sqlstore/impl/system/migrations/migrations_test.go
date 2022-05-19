package migrations_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/tests"
)

func TestMultichainMigration(t *testing.T) {
	url := tests.PostgresURLWithImage(t, "textile/tableland-postgres", "20220504_110247", "tableland")
	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	require.NoError(t, err)
	defer pool.Close()

	var execQueryCount = func(query string) int {
		row := pool.QueryRow(ctx, query)
		var oldNames int
		err = row.Scan(&oldNames)
		require.NoError(t, err)
		return oldNames
	}

	// 1. Check that we have tables with the old (non-chainID scoped) format.
	queryOldFormat :=
		`SELECT count(*) FROM information_schema.tables WHERE table_name ~ '^_[0-9]+$' AND table_type='BASE TABLE'`
	oldNamesCountBeforeMigration := execQueryCount(queryOldFormat)
	require.Equal(t, 518, oldNamesCountBeforeMigration)

	// 2. Boostrap system store to run the db migrations.
	_, err = system.New(url, tableland.ChainID(1337))
	require.NoError(t, err)

	// 3. Check that:
	//    - We don't have *any* table with the old format.
	//    - We have exactly the same amount of different tables with the new format as we had in point 1.
	oldNamesCountAfterMigration := execQueryCount(queryOldFormat)
	require.Equal(t, 0, oldNamesCountAfterMigration)

	queryNewFormat :=
		`SELECT count(*) FROM information_schema.tables WHERE table_name ~ '^\w+_4_[0-9]+$' and table_type='BASE TABLE'`
	newNamesCountAfterMigration := execQueryCount(queryNewFormat)
	require.Equal(t, oldNamesCountBeforeMigration, newNamesCountAfterMigration)
}
