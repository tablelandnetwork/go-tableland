package impl

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/database"
	"github.com/textileio/go-tableland/pkg/database/db"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

// EventFeedStore is the storage layer for EventFeed.
type EventFeedStore struct {
	db *database.SQLiteDB
}

var _ eventfeed.EventFeedStore = (*EventFeedStore)(nil)

// NewEventFeedStore creates a new feed store.
func NewEventFeedStore(db *database.SQLiteDB) *EventFeedStore {
	return &EventFeedStore{
		db: db,
	}
}

// Begin starts a tx.
func (s *EventFeedStore) Begin() (*sql.Tx, error) {
	return s.db.DB.Begin()
}

// WithTx returns an EventFeedStore with a tx attached.
func (s *EventFeedStore) WithTx(tx *sql.Tx) eventfeed.EventFeedStore {
	return &EventFeedStore{
		db: &database.SQLiteDB{
			URI:     s.db.URI,
			DB:      s.db.DB,
			Queries: s.db.Queries.WithTx(tx),
			Log:     s.db.Log,
		},
	}
}

// AreEVMEventsPersisted returns true if there're events persisted for the provided txn hash, and false otherwise.
func (s *EventFeedStore) AreEVMEventsPersisted(
	ctx context.Context, chainID tableland.ChainID, txnHash common.Hash,
) (bool, error) {
	params := db.AreEVMEventsPersistedParams{
		ChainID: int64(chainID),
		TxHash:  txnHash.Hex(),
	}

	_, err := s.db.Queries.AreEVMEventsPersisted(ctx, params)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("evm txn events lookup: %s", err)
	}
	return true, nil
}

// SaveEVMEvents saves the provider EVMEvents.
func (s *EventFeedStore) SaveEVMEvents(
	ctx context.Context, chainID tableland.ChainID, events []eventfeed.EVMEvent,
) error {
	queries := s.db.Queries
	for _, e := range events {
		args := db.InsertEVMEventParams{
			ChainID:     int64(chainID),
			EventJson:   string(e.EventJSON),
			EventType:   e.EventType,
			Address:     e.Address.Hex(),
			Topics:      string(e.Topics),
			Data:        e.Data,
			BlockNumber: int64(e.BlockNumber),
			TxHash:      e.TxHash.Hex(),
			TxIndex:     e.TxIndex,
			BlockHash:   e.BlockHash.Hex(),
			EventIndex:  e.Index,
		}
		if err := queries.InsertEVMEvent(ctx, args); err != nil {
			return fmt.Errorf("insert evm event: %s", err)
		}
	}

	return nil
}

// GetBlocksMissingExtraInfo returns a list of block numbers that don't contain enhanced information.
// It receives an optional fromHeight to only look for blocks after a block number. If null it will look
// for blocks at any height.
func (s *EventFeedStore) GetBlocksMissingExtraInfo(
	ctx context.Context, chainID tableland.ChainID, lastKnownHeight *int64,
) ([]int64, error) {
	var blockNumbers []int64
	var err error
	if lastKnownHeight == nil {
		blockNumbers, err = s.db.Queries.GetBlocksMissingExtraInfo(ctx, int64(chainID))
	} else {
		params := db.GetBlocksMissingExtraInfoByBlockNumberParams{
			ChainID:     int64(chainID),
			BlockNumber: *lastKnownHeight,
		}
		blockNumbers, err = s.db.Queries.GetBlocksMissingExtraInfoByBlockNumber(ctx, params)
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get blocks missing extra info: %s", err)
	}

	return blockNumbers, nil
}

// InsertBlockExtraInfo inserts enhanced information for a block.
func (s *EventFeedStore) InsertBlockExtraInfo(
	ctx context.Context, chainID tableland.ChainID, blockNumber int64, timestamp uint64,
) error {
	params := db.InsertBlockExtraInfoParams{
		ChainID:     int64(chainID),
		BlockNumber: blockNumber,
		Timestamp:   int64(timestamp),
	}
	if err := s.db.Queries.InsertBlockExtraInfo(ctx, params); err != nil {
		return fmt.Errorf("insert block extra info: %s", err)
	}

	return nil
}

// GetEVMEvents returns all the persisted events for a transaction.
func (s *EventFeedStore) GetEVMEvents(
	ctx context.Context, chainID tableland.ChainID, txnHash common.Hash,
) ([]eventfeed.EVMEvent, error) {
	args := db.GetEVMEventsParams{
		ChainID: int64(chainID),
		TxHash:  txnHash.Hex(),
	}
	events, err := s.db.Queries.GetEVMEvents(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("get events by txhash: %s", err)
	}

	ret := make([]eventfeed.EVMEvent, len(events))
	for i, event := range events {
		ret[i] = eventfeed.EVMEvent{
			Address:     common.HexToAddress(event.Address),
			Topics:      []byte(event.Topics),
			Data:        event.Data,
			BlockNumber: uint64(event.BlockNumber),
			TxHash:      common.HexToHash(event.TxHash),
			TxIndex:     event.TxIndex,
			BlockHash:   common.HexToHash(event.BlockHash),
			Index:       event.EventIndex,
			ChainID:     tableland.ChainID(event.ChainID),
			EventJSON:   []byte(event.EventJson),
			EventType:   event.EventType,
		}
	}

	return ret, nil
}

// GetBlockExtraInfo info returns stored information about an EVM block.
func (s *EventFeedStore) GetBlockExtraInfo(
	ctx context.Context, chainID tableland.ChainID, blockNumber int64,
) (eventfeed.EVMBlockInfo, error) {
	params := db.GetBlockExtraInfoParams{
		ChainID:     int64(chainID),
		BlockNumber: blockNumber,
	}

	blockInfo, err := s.db.Queries.GetBlockExtraInfo(ctx, params)
	if err == sql.ErrNoRows {
		return eventfeed.EVMBlockInfo{}, fmt.Errorf("block information not found: %w", err)
	}
	if err != nil {
		return eventfeed.EVMBlockInfo{}, fmt.Errorf("get block information: %s", err)
	}

	return eventfeed.EVMBlockInfo{
		ChainID:     tableland.ChainID(blockInfo.ChainID),
		BlockNumber: blockInfo.BlockNumber,
		Timestamp:   time.Unix(blockInfo.Timestamp, 0),
	}, nil
}

// InstrutmentedEventFeedStore is the intrumented storage layer for EventFeed.
type InstrutmentedEventFeedStore struct {
	store            eventfeed.EventFeedStore
	callCount        instrument.Int64Counter
	latencyHistogram instrument.Int64Histogram
}

var _ eventfeed.EventFeedStore = (*InstrutmentedEventFeedStore)(nil)

// NewInstrumentedEventFeedStore creates a new feed store.
func NewInstrumentedEventFeedStore(db *database.SQLiteDB) (*InstrutmentedEventFeedStore, error) {
	meter := global.MeterProvider().Meter("tableland")
	callCount, err := meter.Int64Counter("tableland.eventfeed.store.call.count")
	if err != nil {
		return &InstrutmentedEventFeedStore{}, fmt.Errorf("registering call counter: %s", err)
	}
	latencyHistogram, err := meter.Int64Histogram("tableland.eventfeed.store.latency")
	if err != nil {
		return &InstrutmentedEventFeedStore{}, fmt.Errorf("registering latency histogram: %s", err)
	}

	return &InstrutmentedEventFeedStore{
		store:            NewEventFeedStore(db),
		callCount:        callCount,
		latencyHistogram: latencyHistogram,
	}, nil
}

// Begin starts a tx.
func (s *InstrutmentedEventFeedStore) Begin() (*sql.Tx, error) {
	return s.store.Begin()
}

// WithTx returns an EventFeedStore with a tx attached.
func (s *InstrutmentedEventFeedStore) WithTx(tx *sql.Tx) eventfeed.EventFeedStore {
	return s.store.WithTx(tx)
}

// AreEVMEventsPersisted returns true if there're events persisted for the provided txn hash, and false otherwise.
func (s *InstrutmentedEventFeedStore) AreEVMEventsPersisted(
	ctx context.Context, chainID tableland.ChainID, txnHash common.Hash,
) (bool, error) {
	start := time.Now()
	ok, err := s.store.AreEVMEventsPersisted(ctx, chainID, txnHash)
	latency := time.Since(start).Milliseconds()

	attributes := append([]attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("AreEVMEventsPersisted")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(chainID))},
	}, metrics.BaseAttrs...)

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return ok, err
}

// SaveEVMEvents saves the provider EVMEvents.
func (s *InstrutmentedEventFeedStore) SaveEVMEvents(
	ctx context.Context, chainID tableland.ChainID, events []eventfeed.EVMEvent,
) error {
	start := time.Now()
	err := s.store.SaveEVMEvents(ctx, chainID, events)
	latency := time.Since(start).Milliseconds()

	attributes := append([]attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("SaveEVMEvents")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(chainID))},
	}, metrics.BaseAttrs...)

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}

// GetBlocksMissingExtraInfo returns a list of block numbers that don't contain enhanced information.
// It receives an optional fromHeight to only look for blocks after a block number. If null it will look
// for blocks at any height.
func (s *InstrutmentedEventFeedStore) GetBlocksMissingExtraInfo(
	ctx context.Context, chainID tableland.ChainID, lastKnownHeight *int64,
) ([]int64, error) {
	start := time.Now()
	blocks, err := s.store.GetBlocksMissingExtraInfo(ctx, chainID, lastKnownHeight)
	latency := time.Since(start).Milliseconds()

	attributes := append([]attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetBlocksMissingExtraInfo")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(chainID))},
	}, metrics.BaseAttrs...)

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return blocks, err
}

// InsertBlockExtraInfo inserts enhanced information for a block.
func (s *InstrutmentedEventFeedStore) InsertBlockExtraInfo(
	ctx context.Context, chainID tableland.ChainID, blockNumber int64, timestamp uint64,
) error {
	start := time.Now()
	err := s.store.InsertBlockExtraInfo(ctx, chainID, blockNumber, timestamp)
	latency := time.Since(start).Milliseconds()

	attributes := append([]attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("InsertBlockExtraInfo")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(chainID))},
	}, metrics.BaseAttrs...)

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}

// GetEVMEvents returns all the persisted events for a transaction.
func (s *InstrutmentedEventFeedStore) GetEVMEvents(
	ctx context.Context, chainID tableland.ChainID, txnHash common.Hash,
) ([]eventfeed.EVMEvent, error) {
	start := time.Now()
	evmEvents, err := s.store.GetEVMEvents(ctx, chainID, txnHash)
	latency := time.Since(start).Milliseconds()

	attributes := append([]attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetEVMEvents")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(chainID))},
	}, metrics.BaseAttrs...)

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return evmEvents, err
}

// GetBlockExtraInfo info returns stored information about an EVM block.
func (s *InstrutmentedEventFeedStore) GetBlockExtraInfo(
	ctx context.Context, chainID tableland.ChainID, blockNumber int64,
) (eventfeed.EVMBlockInfo, error) {
	start := time.Now()
	blockInfo, err := s.store.GetBlockExtraInfo(ctx, chainID, blockNumber)
	latency := time.Since(start).Milliseconds()

	attributes := append([]attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetBlockExtraInfo")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(chainID))},
	}, metrics.BaseAttrs...)

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return blockInfo, err
}
