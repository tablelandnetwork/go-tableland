package impl

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"

	"github.com/textileio/go-tableland/pkg/sharedmemory"

	tbleth "github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.uber.org/atomic"
)

const (
	maxBlocksFetchSizeStart = 100_000
)

// EventFeed provides a stream of filtered events from a SC.
type EventFeed struct {
	log                zerolog.Logger
	store              eventfeed.EventFeedStore
	chainID            tableland.ChainID
	ethClient          eventfeed.ChainClient
	scAddress          common.Address
	scABI              *abi.ABI
	config             *eventfeed.Config
	maxBlocksFetchSize int

	// Shared memory
	sm *sharedmemory.SharedMemory

	// Metrics
	mBaseLabels       []attribute.KeyValue
	mEventTypeCounter instrument.Int64Counter
	mCurrentHeight    atomic.Int64
}

// New returns a new EventFeed.
func New(
	store eventfeed.EventFeedStore,
	chainID tableland.ChainID,
	ethClient eventfeed.ChainClient,
	scAddress common.Address,
	sm *sharedmemory.SharedMemory,
	opts ...eventfeed.Option,
) (*EventFeed, error) {
	config := eventfeed.DefaultConfig()
	for _, o := range opts {
		if err := o(config); err != nil {
			return nil, fmt.Errorf("applying provided option: %s", err)
		}
	}
	scABI, err := tbleth.ContractMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("get contract-abi: %s", err)
	}
	log := logger.With().
		Str("component", "eventfeed").
		Int64("chain_id", int64(chainID)).
		Logger()
	ef := &EventFeed{
		sm:                 sm,
		log:                log,
		store:              store,
		chainID:            chainID,
		ethClient:          ethClient,
		scAddress:          scAddress,
		scABI:              scABI,
		config:             config,
		maxBlocksFetchSize: maxBlocksFetchSizeStart,
	}
	if err := ef.initMetrics(chainID); err != nil {
		return nil, fmt.Errorf("initializing metrics instruments: %s", err)
	}

	return ef, nil
}

// Start sends a stream of filtered events from a smart contract since `fromHeight` to the provided channel.
// This is a blocking call, which the caller must cancel the provided context to shut down gracefully the feed.
// The received channel won't be closed.
func (ef *EventFeed) Start(
	ctx context.Context,
	fromHeight int64,
	ch chan<- eventfeed.BlockEvents,
	filterEventTypes []eventfeed.EventType,
) error {
	ef.log.Debug().Msg("starting...")
	defer ef.log.Debug().Msg("stopped")

	if ef.config.FetchExtraBlockInfo {
		go ef.fetchExtraBlockInfo(ctx)
	}

	// Spinup a background process that will post to chHeads when a new block is detected.
	// This channel will be the heart-beat to pull new logs from the chain.
	// We defer the ctx cancelation to be sure we always gracefully close this background go routine
	// in any event that returns this function.
	ctx, cls := context.WithCancel(ctx)
	defer cls()
	chHeads := make(chan *types.Header, 1)
	if err := ef.notifyNewBlocks(ctx, chHeads); err != nil {
		return fmt.Errorf("creating background head notificator: %s", err)
	}

	// Create filterTopics that will be used to only listening for the desired events.
	filterTopics, err := ef.getTopicsForEventTypes(filterEventTypes)
	if err != nil {
		return fmt.Errorf("creating topics for filtered event types: %s", err)
	}

	// Listen for new blocks, and get new events.
	for h := range chHeads {
		if h.Number.Int64()%100 == 0 {
			ef.log.Debug().
				Int64("height", h.Number.Int64()).
				Int64("max_blocks_fetch_size", int64(ef.maxBlocksFetchSize)).
				Msg("received new chain header")
		}
		// We do a for loop since we'll try to catch from fromHeight to the new reported
		// head in batches with max size MaxEventsBatchSize. This is important to
		// avoid asking the API for very big ranges (e.g: newHead - fromHeight > 100k) since
		// that could would be too big (i.e: make the API fail, the response would be too big,
		// and would consume too much memory).
	Loop:
		for {
			if ctx.Err() != nil {
				break
			}
			// Recall that we only accept as "final" blocks the one that are at least
			// minChainDepth behind the new known head. This is done to avoid reorgs
			// sideffects.
			toHeight := h.Number.Int64() - int64(ef.config.MinBlockChainDepth)
			if toHeight < fromHeight {
				ef.log.Warn().
					Int64("from_height", fromHeight).
					Int64("to_height", toHeight).
					Msgf("from_height bigger than to_height")
				break
			}

			ef.sm.SetLastSeenBlockNumber(ef.chainID, toHeight)

			if toHeight-fromHeight+1 > int64(ef.maxBlocksFetchSize) {
				toHeight = fromHeight + int64(ef.maxBlocksFetchSize) - 1
			}

			// Ask for the desired events between fromHeight to toHeight.
			query := ethereum.FilterQuery{
				FromBlock: big.NewInt(fromHeight),
				ToBlock:   big.NewInt(toHeight),
				Addresses: []common.Address{ef.scAddress},
				Topics:    [][]common.Hash{filterTopics},
			}

			logs, err := ef.filterLogs(ctx, query)
			if err != nil {
				// If we got an error here, log it but allow to be retried
				// in the next head. Probably the API can have transient unavailability.
				ef.log.Warn().Err(err).Msgf("filter logs from %d to %d", fromHeight, toHeight)
				if strings.Contains(err.Error(), "read limit exceeded") ||
					strings.Contains(err.Error(), "Log response size exceeded") ||
					strings.Contains(err.Error(), "is greater than the limit") ||
					strings.Contains(err.Error(), "eth_getLogs and eth_newFilter are limited to a 10,000 blocks range") ||
					strings.Contains(err.Error(), "block range is too wide") {
					ef.maxBlocksFetchSize = ef.maxBlocksFetchSize * 80 / 100
				} else {
					// If we get a "looksback" error it means that history is not available
					// for this chain. It happens in Filecoin based chains, where the
					// history is not available in non archive nodes.
					// In this case, we just move the fromHeight
					// to the current head minus 1995 blocks and ignore the past events.
					// This is temporary until we have a better access to archive nodes.
					if strings.Contains(err.Error(), "lookbacks of more than") {
						fromHeight = h.Number.Int64() - 1995
						ef.log.Warn().
							Err(err).
							Msgf("encountered lookbacks error, moving forward to", fromHeight)
						break
					}

					time.Sleep(ef.config.ChainAPIBackoff)
				}
				continue Loop
			}

			// Remove duplicated logs (needed for Filecoin based chains)
			uniqueLogs := ef.removeDuplicateLogs(logs)

			if len(uniqueLogs) > 0 {
				events := make([]interface{}, len(uniqueLogs))
				for i, l := range uniqueLogs {
					events[i], err = ef.parseEvent(l)
					if err != nil {
						ef.log.
							Error().
							Str("txn_hash", l.TxHash.Hex()).
							Err(err).
							Msg("parsing event")
						time.Sleep(ef.config.ChainAPIBackoff)
						continue Loop
					}
				}

				if ef.config.PersistEvents {
					if err := ef.persistEvents(ctx, uniqueLogs, events); err != nil {
						ef.log.
							Error().
							Err(err).
							Msg("persist events")
						time.Sleep(ef.config.ChainAPIBackoff)
						continue Loop
					}
				}

				blocksEvents := ef.packEvents(uniqueLogs, events)
				for i := range blocksEvents {
					ch <- *blocksEvents[i]
				}
			}

			// Update our fromHeight to the latest processed height plus one.
			fromHeight = toHeight + 1
			ef.mCurrentHeight.Store(fromHeight)
			ef.log.Debug().
				Int64("height", fromHeight).
				Str("progress", fmt.Sprintf("%d%%", fromHeight*100/h.Number.Int64())).
				Msg("processing height")
		}
	}
	return nil
}

func (ef *EventFeed) filterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	logs, err := ef.ethClient.FilterLogs(ctx, query)
	if err != nil {
		return []types.Log{}, fmt.Errorf("filter logs: %s", err)
	}
	return logs, err
}

// removeDuplicateLogs removes duplicate logs from the list of logs
// This is needed because some node RPC endpoints can return duplicate logs
// for a given block range. This is a known bug in FVM and impacts Filecoin
// and Filecoin Calibration nodes.
// See: https://github.com/filecoin-project/ref-fvm/issues/1350
func (ef *EventFeed) removeDuplicateLogs(logs []types.Log) []types.Log {
	seenLogs := make(map[string]bool)
	var uniqueLogs []types.Log
	for l := range logs {
		uniqueLogID := fmt.Sprintf("%d%s%d", logs[l].BlockNumber, logs[l].TxHash.String(), logs[l].Index)

		// skip duplicate logs
		if _, ok := seenLogs[uniqueLogID]; ok {
			ef.log.Warn().
				Int64("block_number", int64(logs[l].BlockNumber)).
				Str("txn_hash", logs[l].TxHash.String()).
				Int64("log_index", int64(logs[l].Index)).
				Msg("removing duplicate logs")
			continue
		}

		seenLogs[uniqueLogID] = true
		uniqueLogs = append(uniqueLogs, logs[l])
	}
	return uniqueLogs
}

// packEvents packs a linear stream of events in two nested groups:
// 1) First, by block_number.
// 2) Within a block_number, by txn_hash.
// Remember that one block contains multiple txns, and each txn can have more than one event.
func (ef *EventFeed) packEvents(logs []types.Log, parsedEvents []interface{}) []*eventfeed.BlockEvents {
	if len(logs) == 0 {
		return nil
	}

	var ret []*eventfeed.BlockEvents
	var newEvents *eventfeed.BlockEvents
	for i, l := range logs {
		// New block number detected? -> Close the block grouping.
		if newEvents == nil || newEvents.BlockNumber != int64(l.BlockNumber) {
			newEvents = &eventfeed.BlockEvents{
				BlockNumber: int64(l.BlockNumber),
			}
			ret = append(ret, newEvents)
		}
		// New txn hash detected? -> Close the txn hash event grouping, and continue with the next.
		if len(newEvents.Txns) == 0 || newEvents.Txns[len(newEvents.Txns)-1].TxnHash.String() != l.TxHash.String() {
			newEvents.Txns = append(newEvents.Txns, eventfeed.TxnEvents{
				TxnHash: l.TxHash,
			})
		}
		newEvents.Txns[len(newEvents.Txns)-1].Events = append(newEvents.Txns[len(newEvents.Txns)-1].Events, parsedEvents[i])
	}

	return ret
}

// parseEvent deconstructs a raw event that was received from the Ethereum node,
// to a structured representation. Since the event can be from different types,
// we return an interface.
// Every possible type in the interface{} is an auto-generated struct by
// `make ethereum` named `Contract*` (e.g: ContractRunSQL, ContractTransfer, etc).
// See this mapping in the `supportedEvents` map global variable in this file.
func (ef *EventFeed) parseEvent(l types.Log) (interface{}, error) {
	// We get an event descriptior from the common.Hash value that is always
	// in Topic[0] in events. This is an ID for the kind of event.
	eventDescr, err := ef.scABI.EventByID(l.Topics[0])
	if err != nil {
		return eventfeed.TxnEvents{}, fmt.Errorf("detecting event type: %s", err)
	}

	se, ok := eventfeed.SupportedEvents[eventfeed.EventType(eventDescr.Name)]
	if !ok {
		return eventfeed.TxnEvents{}, fmt.Errorf("unknown event type %s", eventDescr.Name)
	}
	// Create a new *ContractXXXX struct that corresponds to this event.
	// e.g: *ContractRunSQL if this event was one fired by runSQL(..) SC function.
	i := reflect.New(se).Interface()

	// Now we unmarshal the event data, to the *ContractXXX struct.
	// First, we unmarshal the information contained in the `data` of the event, which
	// are non-indexed fields of the event.
	if len(l.Data) > 0 {
		if err := ef.scABI.UnpackIntoInterface(i, eventDescr.Name, l.Data); err != nil {
			return eventfeed.TxnEvents{}, fmt.Errorf("unpacking into interface: %s", err)
		}
	}
	// Second, we unmarshal indexed fields which aren't in data but in Topics[1:].
	var indexed abi.Arguments
	for _, arg := range eventDescr.Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	if err := abi.ParseTopics(i, indexed, l.Topics[1:]); err != nil {
		return eventfeed.TxnEvents{}, fmt.Errorf("unpacking indexed topics: %s", err)
	}
	// Note that the above two steps of unmarshalling isn't something particular
	// to us, it's just how Ethereum works.
	// This parsedEvent(...) function was coded in a generic way, so it will hardly
	// ever change.

	attrs := append([]attribute.KeyValue{attribute.String("name", eventDescr.Name)}, ef.mBaseLabels...)
	ef.mEventTypeCounter.Add(context.Background(), 1, attrs...)

	return i, nil
}

func (ef *EventFeed) getTopicsForEventTypes(ets []eventfeed.EventType) ([]common.Hash, error) {
	for _, fet := range ets {
		if _, ok := eventfeed.SupportedEvents[fet]; !ok {
			return nil, fmt.Errorf("event type filter %s isn't supported", fet)
		}
	}
	topics := make([]common.Hash, len(ets))
	for i, et := range ets {
		e, ok := ef.scABI.Events[string(et)]
		if !ok {
			return nil, fmt.Errorf("event type %s wasn't found in compiled contract", et)
		}
		topics[i] = e.ID
	}
	return topics, nil
}

// notifyNewBlocks will send to the provided channel new detected blocks in the chain.
// It's mandatory that the caller cancels the provided context to gracefully close the background process.
// When this happens the provided channel will be closed.
func (ef *EventFeed) notifyNewBlocks(ctx context.Context, clientCh chan *types.Header) error {
	// Always push as fast as possible the latest block.
	ctx2, cls := context.WithTimeout(ctx, time.Second*10)
	defer cls()
	h, err := ef.ethClient.HeaderByNumber(ctx2, nil)
	if err != nil {
		return fmt.Errorf("get current block: %s", err)
	}
	clientCh <- h

	go func() {
		defer close(clientCh)

		for {
			select {
			case <-ctx.Done():
				ef.log.Info().Msg("gracefully closing new blocks polling")
				return
			case <-time.After(ef.config.NewHeadPollFreq):
				ctx, cls := context.WithTimeout(ctx, time.Second*10)
				h, err := ef.ethClient.HeaderByNumber(ctx, nil)
				if err != nil {
					ef.log.Error().Err(err).Msg("get latest block")
				} else {
					clientCh <- h
				}
				cls()
			}
		}
	}()

	return nil
}

func (ef *EventFeed) persistEvents(ctx context.Context, events []types.Log, parsedEvents []interface{}) error {
	// All Contract* auto-generated structs contain the `Raw` field which we wan't to avoid appearing in the JSON
	// serialization. The only thing we know about events is that they're interface{}.
	// We can't use `json:"-"` because Contract* is auto-generated so we can't easily edit the struct tags.
	//
	// We use jsoniter to dynamically configure the Marshal(...) function to omit any field named `Raw` dynamically.
	// This is exactly what we need.
	cfg := jsoniter.Config{}.Froze()
	cfg.RegisterExtension(&omitRawFieldExtension{})

	tx, err := ef.store.Begin()
	if err != nil {
		return fmt.Errorf("opening db tx: %s", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			ef.log.Error().Err(err).Msg("persist events rollback txn")
		}
	}()

	store := ef.store.WithTx(tx)

	persistedTxnHashEvents := map[common.Hash]bool{}
	tblEvents := make([]eventfeed.EVMEvent, 0, len(events))
	for i, e := range events {
		// If we already have registered events for the TxHash, we skip persisting this event.
		// This means that one of two things happened:
		// - We persisted the events before, and the validator closed without EventProcessor executing them; so, in the
		//   next restart these events will be fetched again and re-tried to be persisted. We're safe to skip that work.
		// - In circumstantces where the validator config allows reorgs, the same EVM txn can appear in a "later" block
		//   again. The way EventProcessor works is by skipping executing this EVM txn if appears later, so the first
		//   time it was seen is the one that counts. For persistence we do the same; only the first time it appeared
		//   is the event information we save to be coherent with execution. In any case, a validator config that allows
		//   reorgs isn't safe for state coherence between validators so this should only happen in test environments.
		if _, ok := persistedTxnHashEvents[e.TxHash]; !ok {
			areTxnHashEventsPersisted, err := store.AreEVMEventsPersisted(ctx, ef.chainID, e.TxHash)
			if err != nil {
				return fmt.Errorf("check if evm txn events are persisted: %s", err)
			}
			persistedTxnHashEvents[e.TxHash] = areTxnHashEventsPersisted
		}
		if persistedTxnHashEvents[e.TxHash] {
			continue
		}

		eventJSONBytes, err := cfg.Marshal(parsedEvents[i])
		if err != nil {
			return fmt.Errorf("marshaling event: %s", err)
		}
		topicsHex := make([]string, len(e.Topics))
		for i, t := range e.Topics {
			topicsHex[i] = t.Hex()
		}
		topicsJSONBytes, err := json.Marshal(topicsHex)
		if err != nil {
			return fmt.Errorf("marshaling topics array: %s", err)
		}
		// The reflect names are *ethereum.XXXXX, so we get only XXXXX.
		eventType := strings.SplitN(reflect.TypeOf(parsedEvents[i]).String(), ".", 2)[1]
		tblEvent := eventfeed.EVMEvent{
			// Direct mapping from types.Log
			Address:     e.Address,
			Topics:      topicsJSONBytes,
			Data:        e.Data,
			BlockNumber: e.BlockNumber,
			TxHash:      e.TxHash,
			TxIndex:     e.TxIndex,
			BlockHash:   e.BlockHash,
			Index:       e.Index,

			// Enhanced fields
			ChainID:   ef.chainID,
			EventJSON: eventJSONBytes,
			EventType: eventType,
		}
		tblEvents = append(tblEvents, tblEvent)
		if err := telemetry.Collect(ctx, toNewTablelandEvent(tblEvent)); err != nil {
			return fmt.Errorf("collecting new tableland event metric: %s", err)
		}
	}

	if err := store.SaveEVMEvents(ctx, ef.chainID, tblEvents); err != nil {
		return fmt.Errorf("persisting events: %s", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit db tx: %s", err)
	}

	return nil
}

// Based on https://github.com/json-iterator/go/issues/392
type omitRawFieldExtension struct {
	jsoniter.DummyExtension
}

func (e *omitRawFieldExtension) UpdateStructDescriptor(structDescriptor *jsoniter.StructDescriptor) {
	if binding := structDescriptor.GetField("Raw"); binding != nil {
		binding.ToNames = []string{}
	}
}

func toNewTablelandEvent(e eventfeed.EVMEvent) telemetry.NewTablelandEventMetric {
	return telemetry.NewTablelandEventMetric{
		Address:     e.Address.String(),
		Topics:      e.Topics,
		Data:        e.Data,
		BlockNumber: e.BlockNumber,
		TxHash:      e.TxHash.String(),
		TxIndex:     e.TxIndex,
		BlockHash:   e.BlockHash.String(),
		Index:       e.Index,
		ChainID:     int64(e.ChainID),
		EventJSON:   string(e.EventJSON),
		EventType:   e.EventType,
	}
}
