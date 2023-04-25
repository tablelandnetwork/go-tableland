package impl

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/database"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/tests"
)

func TestEVMEventPersistence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbURI := tests.Sqlite3URI(t)

	chainID := tableland.ChainID(1337)

	db, err := database.Open(dbURI)
	require.NoError(t, err)

	store := NewEventFeedStore(db)

	testData := []eventfeed.EVMEvent{
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
		exists, err := store.AreEVMEventsPersisted(ctx, chainID, event.TxHash)
		require.NoError(t, err)
		require.False(t, exists)
	}

	err = store.SaveEVMEvents(ctx, chainID, testData)
	require.NoError(t, err)

	// Check that AreEVMEventsPersisted for the future txn hashes are found, and the data matches.
	for _, event := range testData {
		exists, err := store.AreEVMEventsPersisted(ctx, chainID, event.TxHash)
		require.NoError(t, err)
		require.True(t, exists)

		events, err := store.GetEVMEvents(ctx, chainID, event.TxHash)
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
