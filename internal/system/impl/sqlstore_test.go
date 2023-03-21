package impl

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
	sys "github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	executor "github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor/impl"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/tests"
)

var chainID = tableland.ChainID(1337)

func TestSystemSQLStoreService(t *testing.T) {
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
	svc, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables", "https://render.tableland.xyz", "")
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

		svc, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables", "", "")
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

		svc, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables", "https://render.tableland.xyz", "")
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

		svc, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables", "https://render.tableland.xyz/", "")
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

		_, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables", "foo", "")
		require.Error(t, err)
		require.ErrorContains(t, err, "metadata renderer uri could not be parsed")
	})

	t.Run("non existent table", func(t *testing.T) {
		t.Parallel()

		svc, err := NewSystemSQLStoreService(stack, "https://tableland.network/tables", "https://render.tableland.xyz", "")
		require.NoError(t, err)

		id, _ := tables.NewTableID("43")
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(ctx, id)
		require.ErrorIs(t, err, sys.ErrTableNotFound)
		require.Equal(t, fmt.Sprintf("https://tableland.network/tables/chain/%d/tables/%s", 1337, id), metadata.ExternalURL)
		require.Equal(t, "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0nNTEyJyBoZWlnaHQ9JzUxMicgeG1sbnM9J2h0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnJz48cmVjdCB3aWR0aD0nNTEyJyBoZWlnaHQ9JzUxMicgZmlsbD0nIzAwMCcvPjwvc3ZnPg==", metadata.Image) // nolint
		require.Equal(t, "Table not found", metadata.Message)
	})

	t.Run("with metadata uri and animation uri", func(t *testing.T) {
		t.Parallel()

		svc, err := NewSystemSQLStoreService(
			stack,
			"https://tableland.network/tables",
			"https://render.tableland.xyz",
			"https://render.tableland.xyz/anim",
		)
		require.NoError(t, err)

		metadata, err := svc.GetTableMetadata(ctx, id)
		require.NoError(t, err)

		require.Equal(t, "foo_1337_42", metadata.Name)
		require.Equal(t, fmt.Sprintf("https://tableland.network/tables/chain/%d/tables/%s", 1337, id), metadata.ExternalURL)
		require.Equal(t, "https://render.tableland.xyz/1337/42", metadata.Image)
		require.Equal(t, "https://render.tableland.xyz/anim/?chain=1337&id=42", metadata.AnimationURL)
		require.Equal(t, "date", metadata.Attributes[0].DisplayType)
		require.Equal(t, "created", metadata.Attributes[0].TraitType)
	})
}

func TestEVMEventPersistence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbURI := tests.Sqlite3URI(t)

	store, err := system.New(dbURI, chainID)
	require.NoError(t, err)

	testData := []tableland.EVMEvent{
		{
			Address:     common.HexToAddress("0x10"),
			Topics:      []byte(`["0x111,"0x122"]`),
			Data:        []byte("data1"),
			BlockNumber: 1,
			TxHash:      common.HexToHash("0x11"),
			TxIndex:     11,
			BlockHash:   common.HexToHash("0x12"),
			Index:       12,
			ChainID:     chainID,
			EventJSON:   []byte("eventjson1"),
			EventType:   "Type1",
		},
		{
			Address:     common.HexToAddress("0x20"),
			Topics:      []byte(`["0x211,"0x222"]`),
			Data:        []byte("data2"),
			BlockNumber: 2,
			TxHash:      common.HexToHash("0x21"),
			TxIndex:     11,
			BlockHash:   common.HexToHash("0x22"),
			Index:       12,
			ChainID:     chainID,
			EventJSON:   []byte("eventjson2"),
			EventType:   "Type2",
		},
	}

	// Check that AreEVMEventsPersisted for the future txn hashes aren't found.
	for _, event := range testData {
		exists, err := store.AreEVMEventsPersisted(ctx, event.TxHash)
		require.NoError(t, err)
		require.False(t, exists)
	}

	err = store.SaveEVMEvents(ctx, testData)
	require.NoError(t, err)

	// Check that AreEVMEventsPersisted for the future txn hashes are found, and the data matches.
	for _, event := range testData {
		exists, err := store.AreEVMEventsPersisted(ctx, event.TxHash)
		require.NoError(t, err)
		require.True(t, exists)

		events, err := store.GetEVMEvents(ctx, event.TxHash)
		require.NoError(t, err)
		require.Len(t, events, 1)

		require.Equal(t, events[0].Address, event.Address)
		require.Equal(t, events[0].Topics, event.Topics)
		require.Equal(t, events[0].Data, event.Data)
		require.Equal(t, events[0].BlockNumber, event.BlockNumber)
		require.Equal(t, events[0].TxHash, event.TxHash)
		require.Equal(t, events[0].TxIndex, event.TxIndex)
		require.Equal(t, events[0].BlockHash, event.BlockHash)
		require.Equal(t, events[0].Index, event.Index)
		require.Equal(t, events[0].ChainID, chainID)
		require.Equal(t, events[0].EventJSON, event.EventJSON)
		require.Equal(t, events[0].EventType, event.EventType)
	}
}
