package system

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/tests"
)

func TestSystemStore(t *testing.T) {
	url, err := tests.PostgresURL()
	require.NoError(t, err)

	ctx := context.Background()
	pool, err := pgxpool.Connect(ctx, url)
	require.NoError(t, err)

	store, err := New(pool)
	require.NoError(t, err)

	_, err = store.GetTable(ctx, uuid.New())
	require.Error(t, err) // no table is found

	tableUUID := uuid.New()
	err = store.InsertTable(ctx, tableUUID, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF")
	require.NoError(t, err)

	table, err := store.GetTable(ctx, tableUUID)
	require.NoError(t, err)

	require.Equal(t, tableUUID, table.UUID)
	require.Equal(t, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", table.Controller)
	require.NotEqual(t, new(time.Time), table.CreatedAt) // CreatedAt is not the zero value
}
