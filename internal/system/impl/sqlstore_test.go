package impl

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/tests"
)

func TestSystemSQLStoreService(t *testing.T) {
	url, err := tests.PostgresURL()
	require.NoError(t, err)

	ctx := context.Background()
	store, err := impl.New(ctx, url, true)
	require.NoError(t, err)

	// populate the system_tables with a table
	tableUUID := uuid.New()
	err = store.InsertTable(ctx, tableUUID, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF")
	require.NoError(t, err)

	svc := NewSystemSQLStoreService(store)
	metadata, err := svc.GetTableMetadata(ctx, tableUUID)
	require.NoError(t, err)

	require.Equal(t, fmt.Sprintf("https://tableland.com/tables/%s", tableUUID.String()), metadata.ExternalURL)
	require.Equal(t, "https://hub.textile.io/thread/bafkqtqxkgt3moqxwa6rpvtuyigaoiavyewo67r3h7gsz4hov2kys7ha/buckets/bafzbeicpzsc423nuninuvrdsmrwurhv3g2xonnduq4gbhviyo5z4izwk5m/todo-list.png", metadata.Image) //nolint
	require.Equal(t, "date", metadata.Attributes[0].DisplayType)
	require.Equal(t, "created", metadata.Attributes[0].TraitType)

	// this is hard to test because the created_at comes from the database. just testing is not the 1970 value
	require.NotEqual(t, new(time.Time).Unix(), metadata.Attributes[0].Value)

	tables, err := svc.GetTablesByController(ctx, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF")
	require.NoError(t, err)
	require.Equal(t, 1, len(tables))
	require.Equal(t, tableUUID, tables[0].UUID)
	require.Equal(t, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", tables[0].Controller)
	require.Equal(t, metadata.Attributes[0].Value, tables[0].CreatedAt.Unix())
}
