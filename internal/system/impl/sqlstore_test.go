package impl

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/router/middlewares"
	"github.com/textileio/go-tableland/internal/tableland"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	txnimpl "github.com/textileio/go-tableland/pkg/txn/impl"
	"github.com/textileio/go-tableland/tests"
)

var chainID = tableland.ChainID(1337)

func TestSystemSQLStoreService(t *testing.T) {
	t.Parallel()

	url := tests.Sqlite3URI()

	ctx := context.WithValue(context.Background(), middlewares.ContextKeyChainID, tableland.ChainID(1337))
	store, err := system.New(url, chainID)
	require.NoError(t, err)

	// populate the registry with a table
	txnp, err := txnimpl.NewTxnProcessor(1337, url, 0, nil)
	require.NoError(t, err)
	b, err := txnp.OpenBatch(ctx)
	require.NoError(t, err)

	parser, err := parserimpl.New([]string{"system_", "registry"})
	require.NoError(t, err)
	id, _ := tableland.NewTableID("42")
	createStmt, err := parser.ValidateCreateTable("create table foo_1337 (bar int)", 1337)
	require.NoError(t, err)

	err = b.InsertTable(ctx, id, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", createStmt)
	require.NoError(t, err)
	require.NoError(t, b.Commit())
	require.NoError(t, b.Close())

	stack := map[tableland.ChainID]sqlstore.SystemStore{1337: store}
	svc, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables")
	require.NoError(t, err)
	metadata, err := svc.GetTableMetadata(ctx, id)
	require.NoError(t, err)

	require.Equal(t, "foo_1337_42", metadata.Name)
	require.Equal(t, fmt.Sprintf("https://tableland.network/tables/chain/%d/tables/%s", 1337, id), metadata.ExternalURL)
	require.Equal(t, "https://bafkreifhuhrjhzbj4onqgbrmhpysk2mop2jimvdvfut6taiyzt2yqzt43a.ipfs.dweb.link", metadata.Image) //nolint
	require.Equal(t, "date", metadata.Attributes[0].DisplayType)
	require.Equal(t, "created", metadata.Attributes[0].TraitType)

	// this is hard to test because the created_at comes from the database. just testing is not the 1970 value
	require.NotEqual(t, new(time.Time).Unix(), metadata.Attributes[0].Value)

	tables, err := svc.GetTablesByController(ctx, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF")
	require.NoError(t, err)
	require.Equal(t, 1, len(tables))
	require.Equal(t, id, tables[0].ID)
	require.Equal(t, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", tables[0].Controller)
	require.Equal(t, "foo", tables[0].Prefix)
	// echo -n bar:INT| shasum -a 256
	require.Equal(t, "5d70b398f938650871dd0d6d421e8d1d0c89fe9ed6c8a817c97e951186da7172", tables[0].Structure)
	require.Equal(t, metadata.Attributes[0].Value, tables[0].CreatedAt.Unix())
}
