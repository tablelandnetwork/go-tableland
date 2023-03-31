package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/gateway"
	"github.com/textileio/go-tableland/internal/router/middlewares"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/database"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	executor "github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor/impl"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/tests"
)

var chainID = tableland.ChainID(1337)

func TestGatewayInitialization(t *testing.T) {
	t.Parallel()

	t.Run("invalid external uri", func(t *testing.T) {
		t.Parallel()

		_, err := gateway.NewGateway(nil, nil, "invalid uri", "", "")
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid external url prefix")
	})

	t.Run("invalid metadata uri", func(t *testing.T) {
		t.Parallel()

		_, err := gateway.NewGateway(nil, nil, "https://tableland.network", "invalid uri", "")
		require.Error(t, err)
		require.ErrorContains(t, err, "metadata renderer uri could not be parsed")
	})

	t.Run("invalid animation uri", func(t *testing.T) {
		t.Parallel()

		_, err := gateway.NewGateway(nil, nil, "https://tableland.network", "https://tables.tableland.xyz", "invalid uri")
		require.Error(t, err)
		require.ErrorContains(t, err, "animation renderer uri could not be parsed")
	})
}

func TestGateway(t *testing.T) {
	dbURI := tests.Sqlite3URI(t)

	ctx := context.WithValue(context.Background(), middlewares.ContextKeyChainID, chainID)

	parser, err := parserimpl.New([]string{"system_", "registry"})
	require.NoError(t, err)

	db, err := database.Open(dbURI, 1)
	require.NoError(t, err)
	// populate the registry with a table
	ex, err := executor.NewExecutor(chainID, db, parser, 0, nil)
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

	svc, err := gateway.NewGateway(
		parser, NewGatewayStore(db, nil), "https://tableland.network", "https://tables.tableland.xyz", "",
	)
	require.NoError(t, err)
	metadata, err := svc.GetTableMetadata(ctx, chainID, id)
	require.NoError(t, err)

	require.Equal(t, "foo_1337_42", metadata.Name)
	require.Equal(t, fmt.Sprintf("https://tableland.network/api/v1/tables/%d/%s", chainID, id), metadata.ExternalURL)
	require.Equal(t, "https://tables.tableland.xyz/1337/42.svg", metadata.Image) //nolint
	require.Equal(t, "date", metadata.Attributes[0].DisplayType)
	require.Equal(t, "created", metadata.Attributes[0].TraitType)
}

func TestGetMetadata(t *testing.T) {
	t.Parallel()

	dbURI := tests.Sqlite3URI(t)

	parser, err := parserimpl.New([]string{"system_", "registry"})
	require.NoError(t, err)

	db, err := database.Open(dbURI, 1)
	require.NoError(t, err)

	// populate the registry with a table
	ex, err := executor.NewExecutor(chainID, db, parser, 0, nil)
	require.NoError(t, err)
	bs, err := ex.NewBlockScope(context.Background(), 0)
	require.NoError(t, err)

	id, _ := tables.NewTableID("42")
	require.NoError(t, err)

	res, err := bs.ExecuteTxnEvents(context.Background(), eventfeed.TxnEvents{
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

	t.Run("empty metadata uri", func(t *testing.T) {
		t.Parallel()

		parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)

		svc, err := gateway.NewGateway(parser, NewGatewayStore(db, nil), "https://tableland.network", "", "")
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(context.Background(), chainID, id)
		require.NoError(t, err)

		require.Equal(t, "foo_1337_42", metadata.Name)
		require.Equal(t, fmt.Sprintf("https://tableland.network/api/v1/tables/%d/%s", chainID, id), metadata.ExternalURL)
		require.Equal(t, gateway.DefaultMetadataImage, metadata.Image)
		require.Equal(t, "date", metadata.Attributes[0].DisplayType)
		require.Equal(t, "created", metadata.Attributes[0].TraitType)
	})

	t.Run("with metadata uri", func(t *testing.T) {
		t.Parallel()

		parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)

		svc, err := gateway.NewGateway(
			parser, NewGatewayStore(db, nil), "https://tableland.network", "https://tables.tableland.xyz", "",
		)
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(context.Background(), chainID, id)
		require.NoError(t, err)

		require.Equal(t, "foo_1337_42", metadata.Name)
		require.Equal(t, fmt.Sprintf("https://tableland.network/api/v1/tables/%d/%s", chainID, id), metadata.ExternalURL)
		require.Equal(t, "https://tables.tableland.xyz/1337/42.svg", metadata.Image)
		require.Equal(t, "date", metadata.Attributes[0].DisplayType)
		require.Equal(t, "created", metadata.Attributes[0].TraitType)
	})

	t.Run("with metadata uri trailing slash", func(t *testing.T) {
		t.Parallel()

		parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)

		svc, err := gateway.NewGateway(
			parser, NewGatewayStore(db, nil), "https://tableland.network", "https://tables.tableland.xyz/", "",
		)
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(context.Background(), chainID, id)
		require.NoError(t, err)

		require.Equal(t, "foo_1337_42", metadata.Name)

		require.Equal(t, fmt.Sprintf("https://tableland.network/api/v1/tables/%d/%s", chainID, id), metadata.ExternalURL)
		require.Equal(t, "https://tables.tableland.xyz/1337/42.svg", metadata.Image)
		require.Equal(t, "date", metadata.Attributes[0].DisplayType)
		require.Equal(t, "created", metadata.Attributes[0].TraitType)
	})

	t.Run("with wrong metadata uri", func(t *testing.T) {
		t.Parallel()

		parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)

		_, err = gateway.NewGateway(parser, NewGatewayStore(db, nil), "https://tableland.network", "foo", "")
		require.Error(t, err)
		require.ErrorContains(t, err, "metadata renderer uri could not be parsed")
	})

	t.Run("non existent table", func(t *testing.T) {
		t.Parallel()

		parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)

		svc, err := gateway.NewGateway(
			parser, NewGatewayStore(db, nil), "https://tableland.network", "https://tables.tableland.xyz", "",
		)
		require.NoError(t, err)

		id, _ := tables.NewTableID("43")
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(context.Background(), chainID, id)
		require.ErrorIs(t, err, gateway.ErrTableNotFound)
		require.Equal(t, fmt.Sprintf("https://tableland.network/api/v1/tables/%d/%s", chainID, id), metadata.ExternalURL)
		require.Equal(t, "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0nNTEyJyBoZWlnaHQ9JzUxMicgeG1sbnM9J2h0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnJz48cmVjdCB3aWR0aD0nNTEyJyBoZWlnaHQ9JzUxMicgZmlsbD0nIzAwMCcvPjwvc3ZnPg==", metadata.Image) // nolint
		require.Equal(t, "Table not found", metadata.Message)
	})

	t.Run("with metadata uri and animation uri", func(t *testing.T) {
		t.Parallel()

		parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
		require.NoError(t, err)

		svc, err := gateway.NewGateway(
			parser,
			NewGatewayStore(db, nil),
			"https://tableland.network",
			"https://tables.tableland.xyz",
			"https://tables.tableland.xyz",
		)
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(context.Background(), chainID, id)
		require.NoError(t, err)

		require.Equal(t, "foo_1337_42", metadata.Name)
		require.Equal(t, fmt.Sprintf("https://tableland.network/api/v1/tables/%d/%s", chainID, id), metadata.ExternalURL)
		require.Equal(t, "https://tables.tableland.xyz/1337/42.svg", metadata.Image)
		require.Equal(t, "https://tables.tableland.xyz/1337/42.html", metadata.AnimationURL)
		require.Equal(t, "date", metadata.Attributes[0].DisplayType)
		require.Equal(t, "created", metadata.Attributes[0].TraitType)
	})
}

func TestQueryConstraints(t *testing.T) {
	t.Parallel()

	dbURI := tests.Sqlite3URI(t)
	db, err := database.Open(dbURI, 1)
	require.NoError(t, err)

	parsingOpts := []parsing.Option{
		parsing.WithMaxReadQuerySize(44),
	}

	parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"}, parsingOpts...)
	require.NoError(t, err)

	t.Run("read-query-size-nok", func(t *testing.T) {
		t.Parallel()

		gateway, err := gateway.NewGateway(
			parser,
			NewGatewayStore(db, nil),
			"https://tableland.network",
			"https://tables.tableland.xyz",
			"https://tables.tableland.xyz",
		)
		require.NoError(t, err)

		_, err = gateway.RunReadQuery(
			context.Background(), "SELECT * FROM foo_1337_1 WHERE bar = 'hello2'",
		) // length of 45 bytes
		require.Error(t, err)
		require.ErrorContains(t, err, "read query size is too long")
	})
}

func TestUserValue(t *testing.T) {
	uv := &gateway.ColumnValue{}

	var in0 int64 = 100
	require.NoError(t, uv.Scan(in0))
	val := uv.Value()
	v0, ok := val.(int64)
	require.True(t, ok)
	require.Equal(t, in0, v0)
	b, err := json.Marshal(uv)
	require.NoError(t, err)
	var out0 int64
	require.NoError(t, json.Unmarshal(b, &out0))
	require.Equal(t, in0, out0)

	in1 := 100.0
	require.NoError(t, uv.Scan(in1))
	val = uv.Value()
	v1, ok := val.(float64)
	require.True(t, ok)
	require.Equal(t, in1, v1)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out1 float64
	require.NoError(t, json.Unmarshal(b, &out1))
	require.Equal(t, in1, out1)

	in2 := true
	require.NoError(t, uv.Scan(in2))
	val = uv.Value()
	v2, ok := val.(bool)
	require.True(t, ok)
	require.Equal(t, in2, v2)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out2 bool
	require.NoError(t, json.Unmarshal(b, &out2))
	require.Equal(t, in2, out2)

	in3 := []byte("hello there")
	require.NoError(t, uv.Scan(in3))
	val = uv.Value()
	v3, ok := val.([]byte)
	require.True(t, ok)
	require.Equal(t, in3, v3)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out3 []byte
	require.NoError(t, json.Unmarshal(b, &out3))
	require.Equal(t, in3, out3)

	in4 := "hello"
	require.NoError(t, uv.Scan(in4))
	val = uv.Value()
	v4, ok := val.(string)
	require.True(t, ok)
	require.Equal(t, in4, v4)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out4 string
	require.NoError(t, json.Unmarshal(b, &out4))
	require.Equal(t, in4, out4)

	in5 := time.Now()
	require.NoError(t, uv.Scan(in5))
	val = uv.Value()
	v5, ok := val.(time.Time)
	require.True(t, ok)
	require.Equal(t, in5, v5)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out5 time.Time
	require.NoError(t, json.Unmarshal(b, &out5))
	require.Equal(t, in5.Unix(), out5.Unix())

	var in6 interface{}
	require.NoError(t, uv.Scan(in6))
	val = uv.Value()
	require.Nil(t, val)
	require.Equal(t, in6, val)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out6 interface{}
	require.NoError(t, json.Unmarshal(b, &out6))
	require.Equal(t, in6, out6)

	in7 := "{ \"hello"
	require.NoError(t, uv.Scan(in7))
	val = uv.Value()
	v7, ok := val.(string)
	require.True(t, ok)
	require.Equal(t, in7, v7)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out7 string
	require.NoError(t, json.Unmarshal(b, &out7))
	require.Equal(t, in7, out7)

	in8 := "[ \"hello"
	require.NoError(t, uv.Scan(in8))
	val = uv.Value()
	v8, ok := val.(string)
	require.True(t, ok)
	require.Equal(t, in8, v8)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	var out8 string
	require.NoError(t, json.Unmarshal(b, &out8))
	require.Equal(t, in8, out8)

	in9 := "{\"name\":\"aaron\"}"
	require.NoError(t, uv.Scan(in9))
	val = uv.Value()
	v9, ok := val.(json.RawMessage)
	require.True(t, ok)
	require.Greater(t, len(v9), 0)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	require.Equal(t, in9, string(b))

	in10 := "[\"one\",\"two\"]"
	require.NoError(t, uv.Scan(in10))
	val = uv.Value()
	v10, ok := val.(json.RawMessage)
	require.True(t, ok)
	require.Greater(t, len(v10), 0)
	b, err = json.Marshal(uv)
	require.NoError(t, err)
	require.Equal(t, in10, string(b))
}
