package gateway

import (
	"context"
	"database/sql"
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
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/tests"
)

var chainID = tableland.ChainID(1337)

func TestGatewayInitialization(t *testing.T) {
	t.Parallel()

	t.Run("invalid external uri", func(t *testing.T) {
		t.Parallel()

		_, err := NewGateway(nil, nil, "invalid uri", "", "")
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid external url prefix")
	})

	t.Run("invalid metadata uri", func(t *testing.T) {
		t.Parallel()

		_, err := NewGateway(nil, nil, "https://tableland.network", "invalid uri", "")
		require.Error(t, err)
		require.ErrorContains(t, err, "metadata renderer uri could not be parsed")
	})

	t.Run("invalid animation uri", func(t *testing.T) {
		t.Parallel()

		_, err := NewGateway(nil, nil, "https://tableland.network", "https://tables.tableland.xyz", "invalid uri")
		require.Error(t, err)
		require.ErrorContains(t, err, "animation renderer uri could not be parsed")
	})
}

func TestGateway(t *testing.T) {
	dbURI := tests.Sqlite3URI(t)

	ctx := context.WithValue(context.Background(), middlewares.ContextKeyChainID, tableland.ChainID(1337))
	store, err := system.New(dbURI, chainID)
	require.NoError(t, err)

	parser, err := parserimpl.New([]string{"system_", "registry"})
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", dbURI)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)

	// populate the registry with a table
	ex, err := executor.NewExecutor(1337, db, parser, 0, nil)
	require.NoError(t, err)
	bs, err := ex.NewBlockScope(ctx, 0)
	require.NoError(t, err)

	id, _ := tables.NewTableID("42")
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

	parser, err = parserimpl.New([]string{"system_", "registry", "sqlite_"})
	require.NoError(t, err)

	stack := map[tableland.ChainID]sqlstore.SystemStore{1337: store}
	svc, err := NewGateway(parser, stack, "https://tableland.network", "https://tables.tableland.xyz", "")
	require.NoError(t, err)
	metadata, err := svc.GetTableMetadata(ctx, id)
	require.NoError(t, err)

	require.Equal(t, "foo_1337_42", metadata.Name)
	require.Equal(t, fmt.Sprintf("https://tableland.network/api/v1/tables/%d/%s", 1337, id), metadata.ExternalURL)
	require.Equal(t, "https://tables.tableland.xyz/1337/42.svg", metadata.Image) //nolint
	require.Equal(t, "date", metadata.Attributes[0].DisplayType)
	require.Equal(t, "created", metadata.Attributes[0].TraitType)

	// this is hard to test because the created_at comes from the database. just testing is not the 1970 value
	require.NotEqual(t, new(time.Time).Unix(), metadata.Attributes[0].Value)
}

func TestGetMetadata(t *testing.T) {
	t.Parallel()

	dbURI := tests.Sqlite3URI(t)

	ctx := context.WithValue(context.Background(), middlewares.ContextKeyChainID, tableland.ChainID(1337))
	store, err := system.New(dbURI, chainID)
	require.NoError(t, err)

	parser, err := parserimpl.New([]string{"system_", "registry"})
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", dbURI)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)

	// populate the registry with a table
	ex, err := executor.NewExecutor(1337, db, parser, 0, nil)
	require.NoError(t, err)
	bs, err := ex.NewBlockScope(ctx, 0)
	require.NoError(t, err)

	id, _ := tables.NewTableID("42")
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

		parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)

		svc, err := NewGateway(parser, stack, "https://tableland.network", "", "")
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(ctx, id)
		require.NoError(t, err)

		require.Equal(t, "foo_1337_42", metadata.Name)
		require.Equal(t, fmt.Sprintf("https://tableland.network/api/v1/tables/%d/%s", 1337, id), metadata.ExternalURL)
		require.Equal(t, DefaultMetadataImage, metadata.Image)
		require.Equal(t, "date", metadata.Attributes[0].DisplayType)
		require.Equal(t, "created", metadata.Attributes[0].TraitType)
	})

	t.Run("with metadata uri", func(t *testing.T) {
		t.Parallel()

		parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)

		svc, err := NewGateway(parser, stack, "https://tableland.network", "https://tables.tableland.xyz", "")
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(ctx, id)
		require.NoError(t, err)

		require.Equal(t, "foo_1337_42", metadata.Name)
		require.Equal(t, fmt.Sprintf("https://tableland.network/api/v1/tables/%d/%s", 1337, id), metadata.ExternalURL)
		require.Equal(t, "https://tables.tableland.xyz/1337/42.svg", metadata.Image)
		require.Equal(t, "date", metadata.Attributes[0].DisplayType)
		require.Equal(t, "created", metadata.Attributes[0].TraitType)
	})

	t.Run("with metadata uri trailing slash", func(t *testing.T) {
		t.Parallel()

		parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)

		svc, err := NewGateway(parser, stack, "https://tableland.network", "https://tables.tableland.xyz/", "")
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(ctx, id)
		require.NoError(t, err)

		require.Equal(t, "foo_1337_42", metadata.Name)
		require.Equal(t, fmt.Sprintf("https://tableland.network/api/v1/tables/%d/%s", 1337, id), metadata.ExternalURL)
		require.Equal(t, "https://tables.tableland.xyz/1337/42.svg", metadata.Image)
		require.Equal(t, "date", metadata.Attributes[0].DisplayType)
		require.Equal(t, "created", metadata.Attributes[0].TraitType)
	})

	t.Run("with wrong metadata uri", func(t *testing.T) {
		t.Parallel()

		parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)

		_, err = NewGateway(parser, stack, "https://tableland.network", "foo", "")
		require.Error(t, err)
		require.ErrorContains(t, err, "metadata renderer uri could not be parsed")
	})

	t.Run("non existent table", func(t *testing.T) {
		t.Parallel()

		parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)

		svc, err := NewGateway(parser, stack, "https://tableland.network", "https://tables.tableland.xyz", "")
		require.NoError(t, err)

		id, _ := tables.NewTableID("43")
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(ctx, id)
		require.ErrorIs(t, err, ErrTableNotFound)
		require.Equal(t, fmt.Sprintf("https://tableland.network/api/v1/tables/%d/%s", 1337, id), metadata.ExternalURL)
		require.Equal(t, "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0nNTEyJyBoZWlnaHQ9JzUxMicgeG1sbnM9J2h0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnJz48cmVjdCB3aWR0aD0nNTEyJyBoZWlnaHQ9JzUxMicgZmlsbD0nIzAwMCcvPjwvc3ZnPg==", metadata.Image) // nolint
		require.Equal(t, "Table not found", metadata.Message)
	})

	t.Run("with metadata uri and animation uri", func(t *testing.T) {
		t.Parallel()

		parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)

		svc, err := NewGateway(
			parser,
			stack,
			"https://tableland.network",
			"https://tables.tableland.xyz",
			"https://tables.tableland.xyz",
		)
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(ctx, id)
		require.NoError(t, err)

		require.Equal(t, "foo_1337_42", metadata.Name)
		require.Equal(t, fmt.Sprintf("https://tableland.network/api/v1/tables/%d/%s", 1337, id), metadata.ExternalURL)
		require.Equal(t, "https://tables.tableland.xyz/1337/42.svg", metadata.Image)
		require.Equal(t, "https://tables.tableland.xyz/1337/42.html", metadata.AnimationURL)
		require.Equal(t, "date", metadata.Attributes[0].DisplayType)
		require.Equal(t, "created", metadata.Attributes[0].TraitType)
	})
}

func TestQueryConstraints(t *testing.T) {
	t.Parallel()

	dbURI := tests.Sqlite3URI(t)

	ctx := context.WithValue(context.Background(), middlewares.ContextKeyChainID, tableland.ChainID(1337))
	store, err := system.New(dbURI, chainID)
	require.NoError(t, err)
	stack := map[tableland.ChainID]sqlstore.SystemStore{1337: store}

	parsingOpts := []parsing.Option{
		parsing.WithMaxReadQuerySize(44),
	}

	parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"}, parsingOpts...)
	require.NoError(t, err)

	t.Run("read-query-size-nok", func(t *testing.T) {
		t.Parallel()

		gateway, err := NewGateway(
			parser,
			stack,
			"https://tableland.network",
			"https://tables.tableland.xyz",
			"https://tables.tableland.xyz",
		)
		require.NoError(t, err)

		_, err = gateway.RunReadQuery(ctx, "SELECT * FROM foo_1337_1 WHERE bar = 'hello2'") // length of 45 bytes
		require.Error(t, err)
		require.ErrorContains(t, err, "read query size is too long")
	})
}
