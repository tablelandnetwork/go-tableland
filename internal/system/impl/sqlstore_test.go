package impl

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/router/middlewares"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	executor "github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor/impl"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/tests"
)

var chainID = tableland.ChainID(1337)

func TestSystemSQLStoreService(t *testing.T) {
	t.Parallel()

	dbURI := tests.Sqlite3URI()

	ctx := context.WithValue(context.Background(), middlewares.ContextKeyChainID, tableland.ChainID(1337))
	store, err := system.New(dbURI, chainID)
	require.NoError(t, err)

	parser, err := parserimpl.New([]string{"system_", "registry"})
	require.NoError(t, err)
	// populate the registry with a table
	ex, err := executor.NewExecutor(1337, dbURI, parser, 0, nil)
	require.NoError(t, err)
	bs, err := ex.NewBlockScope(ctx, 0)
	require.NoError(t, err)

	id, _ := tableland.NewTableID("42")
	require.NoError(t, err)

	res, err := bs.ExecuteTxnEvents(ctx, eventfeed.TxnEvents{
		TxnHash: common.HexToHash("0x0"),
		Events: []interface{}{
			&ethereum.ContractCreateTable{
				TableId:   big.NewInt(42),
				Owner:     common.HexToAddress("0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF"),
				Statement: "create table foo_1337 (bar int)",
			},
		},
	})
	require.NoError(t, err)
	require.Nil(t, res.Error)
	require.Nil(t, res.ErrorEventIdx)
	require.NoError(t, bs.Commit())
	require.NoError(t, bs.Close())

	stack := map[tableland.ChainID]sqlstore.SystemStore{1337: store}
	svc, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables", "https://render.tableland.xyz")
	require.NoError(t, err)
	metadata, err := svc.GetTableMetadata(ctx, id)
	require.NoError(t, err)

	require.Equal(t, "foo_1337_42", metadata.Name)
	require.Equal(t, fmt.Sprintf("https://tableland.network/tables/chain/%d/tables/%s", 1337, id), metadata.ExternalURL)
	require.Equal(t, "https://render.tableland.xyz/1337/42", metadata.Image) //nolint
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

	tables, err = svc.GetTablesByStructure(ctx, "5d70b398f938650871dd0d6d421e8d1d0c89fe9ed6c8a817c97e951186da7172")
	require.NoError(t, err)
	require.Equal(t, 1, len(tables))
	require.Equal(t, id, tables[0].ID)
	require.Equal(t, "0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF", tables[0].Controller)
	require.Equal(t, "foo", tables[0].Prefix)
	// echo -n bar:INT| shasum -a 256
	require.Equal(t, "5d70b398f938650871dd0d6d421e8d1d0c89fe9ed6c8a817c97e951186da7172", tables[0].Structure)
}

func TestGetSchemaByTableName(t *testing.T) {
	t.Parallel()

	dbURI := tests.Sqlite3URI()

	ctx := context.WithValue(context.Background(), middlewares.ContextKeyChainID, tableland.ChainID(1337))
	store, err := system.New(dbURI, chainID)
	require.NoError(t, err)

	parser, err := parserimpl.New([]string{"system_", "registry"})
	require.NoError(t, err)

	// populate the registry with a table
	ex, err := executor.NewExecutor(1337, dbURI, parser, 0, nil)
	require.NoError(t, err)
	bs, err := ex.NewBlockScope(ctx, 0)
	require.NoError(t, err)

	res, err := bs.ExecuteTxnEvents(ctx, eventfeed.TxnEvents{
		TxnHash: common.HexToHash("0x0"),
		Events: []interface{}{
			&ethereum.ContractCreateTable{
				TableId:   big.NewInt(42),
				Owner:     common.HexToAddress("0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF"),
				Statement: "create table foo_1337 (a int primary key, b text not null default 'foo' unique, check (a > 0))",
			},
		},
	})
	require.NoError(t, err)
	require.Nil(t, res.Error)
	require.Nil(t, res.ErrorEventIdx)
	require.NoError(t, bs.Commit())
	require.NoError(t, bs.Close())

	stack := map[tableland.ChainID]sqlstore.SystemStore{1337: store}
	svc, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables", "https://render.tableland.xyz")
	require.NoError(t, err)

	schema, err := svc.GetSchemaByTableName(ctx, "foo_1337_42")
	require.NoError(t, err)
	require.Len(t, schema.Columns, 2)
	require.Len(t, schema.TableConstraints, 1)

	require.Equal(t, "a", schema.Columns[0].Name)
	require.Equal(t, "int", schema.Columns[0].Type)
	require.Len(t, schema.Columns[0].Constraints, 1)
	require.Equal(t, "PRIMARY KEY", schema.Columns[0].Constraints[0])

	require.Equal(t, "b", schema.Columns[1].Name)
	require.Equal(t, "text", schema.Columns[1].Type)
	require.Len(t, schema.Columns[1].Constraints, 3)
	require.Equal(t, "NOT NULL", schema.Columns[1].Constraints[0])
	require.Equal(t, "DEFAULT 'foo'", schema.Columns[1].Constraints[1])
	require.Equal(t, "UNIQUE", schema.Columns[1].Constraints[2])

	require.Equal(t, "CHECK(a > 0)", schema.TableConstraints[0])
}

func TestGetMetadata(t *testing.T) {
	t.Parallel()

	dbURI := tests.Sqlite3URI()

	ctx := context.WithValue(context.Background(), middlewares.ContextKeyChainID, tableland.ChainID(1337))
	store, err := system.New(dbURI, chainID)
	require.NoError(t, err)

	parser, err := parserimpl.New([]string{"system_", "registry"})
	require.NoError(t, err)
	// populate the registry with a table
	ex, err := executor.NewExecutor(1337, dbURI, parser, 0, nil)
	require.NoError(t, err)
	bs, err := ex.NewBlockScope(ctx, 0)
	require.NoError(t, err)

	id, _ := tableland.NewTableID("42")
	require.NoError(t, err)

	res, err := bs.ExecuteTxnEvents(ctx, eventfeed.TxnEvents{
		TxnHash: common.HexToHash("0x0"),
		Events: []interface{}{
			&ethereum.ContractCreateTable{
				TableId:   big.NewInt(42),
				Owner:     common.HexToAddress("0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF"),
				Statement: "create table foo_1337 (bar int)",
			},
		},
	})
	require.NoError(t, err)
	require.Nil(t, res.Error)
	require.Nil(t, res.ErrorEventIdx)
	require.NoError(t, bs.Commit())
	require.NoError(t, bs.Close())

	stack := map[tableland.ChainID]sqlstore.SystemStore{1337: store}

	t.Run("empty metadata uri", func(t *testing.T) {
		t.Parallel()

		svc, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables", "")
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(ctx, id)
		require.NoError(t, err)

		require.Equal(t, "foo_1337_42", metadata.Name)
		require.Equal(t, fmt.Sprintf("https://tableland.network/tables/chain/%d/tables/%s", 1337, id), metadata.ExternalURL)
		require.Equal(t, DefaultMetadataImage, metadata.Image)
		require.Equal(t, "date", metadata.Attributes[0].DisplayType)
		require.Equal(t, "created", metadata.Attributes[0].TraitType)
	})

	t.Run("with metadata uri", func(t *testing.T) {
		t.Parallel()

		svc, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables", "https://render.tableland.xyz")
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(ctx, id)
		require.NoError(t, err)

		require.Equal(t, "foo_1337_42", metadata.Name)
		require.Equal(t, fmt.Sprintf("https://tableland.network/tables/chain/%d/tables/%s", 1337, id), metadata.ExternalURL)
		require.Equal(t, "https://render.tableland.xyz/1337/42", metadata.Image)
		require.Equal(t, "date", metadata.Attributes[0].DisplayType)
		require.Equal(t, "created", metadata.Attributes[0].TraitType)
	})

	t.Run("with metadata uri trailing slash", func(t *testing.T) {
		t.Parallel()

		svc, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables", "https://render.tableland.xyz/")
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(ctx, id)
		require.NoError(t, err)

		require.Equal(t, "foo_1337_42", metadata.Name)
		require.Equal(t, fmt.Sprintf("https://tableland.network/tables/chain/%d/tables/%s", 1337, id), metadata.ExternalURL)
		require.Equal(t, "https://render.tableland.xyz/1337/42", metadata.Image)
		require.Equal(t, "date", metadata.Attributes[0].DisplayType)
		require.Equal(t, "created", metadata.Attributes[0].TraitType)
	})

	t.Run("with wrong metadata uri", func(t *testing.T) {
		t.Parallel()

		svc, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables", "foo")
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(ctx, id)
		require.NoError(t, err)

		require.Equal(t, "foo_1337_42", metadata.Name)
		require.Equal(t, fmt.Sprintf("https://tableland.network/tables/chain/%d/tables/%s", 1337, id), metadata.ExternalURL)
		require.Equal(t, DefaultMetadataImage, metadata.Image)
		require.Equal(t, "date", metadata.Attributes[0].DisplayType)
		require.Equal(t, "created", metadata.Attributes[0].TraitType)
	})

	t.Run("non existent table", func(t *testing.T) {
		t.Parallel()

		svc, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables", "foo")
		require.NoError(t, err)

		id, _ := tableland.NewTableID("43")
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(ctx, id)
		require.NoError(t, err)

		require.Equal(t, fmt.Sprintf("https://tableland.network/tables/chain/%d/tables/%s", 1337, id), metadata.ExternalURL)
		require.Equal(t, "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0nNTEyJyBoZWlnaHQ9JzUxMicgeG1sbnM9J2h0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnJz48cmVjdCB3aWR0aD0nNTEyJyBoZWlnaHQ9JzUxMicgZmlsbD0nIzAwMCcvPjwvc3ZnPg==", metadata.Image) // nolint
		require.Equal(t, "Table not found", metadata.Message)
	})
}
