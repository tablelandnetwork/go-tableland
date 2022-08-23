package impl

import (
	"context"
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
	"github.com/textileio/go-tableland/pkg/sqlstore"
	tbleth "github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.uber.org/atomic"
)

const (
	maxBlocksFetchSizeStart = 100_000
)

// EventFeed provides a stream of filtered events from a SC.
type EventFeed struct {
	log                zerolog.Logger
	systemStore        sqlstore.SystemStore
	chainID            tableland.ChainID
	ethClient          eventfeed.ChainClient
	scAddress          common.Address
	scABI              *abi.ABI
	config             *eventfeed.Config
	maxBlocksFetchSize int

	// Metrics
	mBaseLabels       []attribute.KeyValue
	mEventTypeCounter syncint64.Counter
	mCurrentHeight    atomic.Int64
}

// New returns a new EventFeed.
func New(
	systemStore sqlstore.SystemStore,
	chainID tableland.ChainID,
	ethClient eventfeed.ChainClient,
	scAddress common.Address,
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
		log:                log,
		systemStore:        systemStore,
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
				break
			}

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
			logs, err := ef.ethClient.FilterLogs(ctx, query)
			if err != nil {
				// If we got an error here, log it but allow to be retried
				// in the next head. Probably the API can have transient unavailability.
				ef.log.Warn().Err(err).Msgf("filter logs from %d to %d", fromHeight, toHeight)
				if strings.Contains(err.Error(), "read limit exceeded") ||
					strings.Contains(err.Error(), "is greater than the limit") {
					ef.maxBlocksFetchSize = ef.maxBlocksFetchSize * 80 / 100
				} else {
					time.Sleep(ef.config.ChainAPIBackoff)
				}
				continue Loop
			}

			if len(logs) > 0 {
				events := make([]interface{}, len(logs))
				for i, l := range logs {
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
					if err := ef.persistEvents(ctx, logs, events); err != nil {
						ef.log.
							Error().
							Err(err).
							Msg("persist events")
						time.Sleep(ef.config.ChainAPIBackoff)
						continue Loop
					}
				}

				blocksEvents, err := ef.packEvents(logs, events)
				if err != nil {
					ef.log.
						Error().
						Err(err).
						Msg("pack events")
					time.Sleep(ef.config.ChainAPIBackoff)
					continue Loop
				}
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

// packEvents packs a linear stream of events in two nested groups:
// 1) First, by block_number.
// 2) Within a block_number, by txn_hash.
// Remember that one block contains multiple txns, and each txn can have more than one event.
func (ef *EventFeed) packEvents(logs []types.Log, parsedEvents []interface{}) ([]*eventfeed.BlockEvents, error) {
	if len(logs) == 0 {
		return nil, nil
	}

	var ret []*eventfeed.BlockEvents
	var new *eventfeed.BlockEvents
	for i, l := range logs {
		// New block number detected? -> Close the block grouping.
		if new == nil || new.BlockNumber != int64(l.BlockNumber) {
			new = &eventfeed.BlockEvents{
				BlockNumber: int64(l.BlockNumber),
			}
			ret = append(ret, new)
		}
		// New txn hash detected? -> Close the txn hash event grouping, and continue with the next.
		if len(new.Txns) == 0 || new.Txns[len(new.Txns)-1].TxnHash.String() != l.TxHash.String() {
			new.Txns = append(new.Txns, eventfeed.TxnEvents{
				TxnHash: l.TxHash,
			})
		}
		new.Txns[len(new.Txns)-1].Events = append(new.Txns[len(new.Txns)-1].Events, parsedEvents[i])
	}

	return ret, nil
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
	hbnCtx, hbnCls := context.WithTimeout(ctx, time.Second*10)
	defer hbnCls()
	h, err := ef.ethClient.HeaderByNumber(hbnCtx, nil)
	if err != nil {
		return fmt.Errorf("get current block: %s", err)
	}
	clientCh <- h

	ch := make(chan *types.Header, 1)
	notifierSignaler := make(chan struct{}, 1)
	notifierSignaler <- struct{}{}
	// Fire a goroutine that relays new detected blocks to the client, while also inspecting
	// the healthiness of the subscription. If the subscription is faulty, it notifies
	// that the subscription should be regenerated.
	go func() {
		defer close(clientCh)
		defer close(notifierSignaler)

		for {
			select {
			case <-ctx.Done():
				ef.log.Info().Msg("gracefully closing new blocks subscription")
				return
			case h := <-ch:
				select {
				case clientCh <- h:
				default:
					ef.log.Warn().Int("height", int(h.Number.Int64())).Msg("dropping new height")
				}
			case <-time.After(ef.config.NewBlockTimeout):
				ef.log.Warn().Dur("timeout", ef.config.NewBlockTimeout).Msgf("new blocks subscription is quiet, rebuilding")
				notifierSignaler <- struct{}{}
			}
		}
	}()

	// This goroutine is responsible to always having a **single** subscription. It can receive a signal from
	// the above goroutine to re-generate the current subscription since it was detected faulty.
	go func() {
		var sub ethereum.Subscription
		for range notifierSignaler {
			if sub != nil {
				sub.Unsubscribe()
			}
			sub, err = ef.ethClient.SubscribeNewHead(ctx, ch)
			if err != nil {
				sub = nil
				ef.log.Error().Err(err).Msg("subscribing to blocks")
				continue
			}
		}
		if sub != nil {
			sub.Unsubscribe()
		}
		ef.log.Info().Msg("gracefully closing notifier")
	}()

	return nil
}

func (ef *EventFeed) persistEvents(ctx context.Context, logs []types.Log, parsedEvents []interface{}) error {
	// All Contract* auto-generated structs contain the `Raw` field which we wan't to avoid appearing in the JSON
	// serialization. The only thing we know about events is that they're interface{}.
	// We can't use `json:"-"` because Contract* is auto-generated so we can't easily edit the struct tags.
	//
	// We use jsoniter to dynamically configure the Marshal(...) function to omit any field named `Raw` dynamically.
	// This is exactly what we need.
	cfg := jsoniter.Config{}.Froze()
	cfg.RegisterExtension(&OmitRawFieldExtension{})

	tx, err := ef.systemStore.Begin(ctx)
	if err != nil {
		return fmt.Errorf("opening db tx: %s", err)
	}
	defer tx.Rollback()

	store := ef.systemStore.WithTx(tx)

	shouldPersistTxnHashEvents := map[common.Hash]bool{}
	blockHeaderCache := map[common.Hash]*types.Header{}
	tblEvents := make([]tableland.EVMEvent, 0, len(logs))
	for i, e := range logs {
		// If we already have registered events for the TxHash, we skip persisting this log.
		// This means that one of two things happened:
		// - We persisted the events before, and the validator closed without EventProcessor executing them; so, in the
		//   next restart these events will be fetched again and re-tried to be persisted. We're safe to skip that work.
		// - In circumstantces where the validator config allows reorgs, the same EVM txn can appear in a "later" block
		//   again. The way EventProcessor works is by skipping executing this EVM txn if appears later, so the first
		//   time it was seen is the one that counts. For persistence we do the same; only the first time it appeared
		//   is the event information we save to be coherent with execution. In any case, a validator config that allows
		//   reorgs isn't safe for state coherence between validators so this should only happen in test environments.
		if _, ok := shouldPersistTxnHashEvents[e.TxHash]; !ok {
			areTxnEventsPersisted, err := store.AreEVMEventsPersisted(ctx, e.TxHash)
			if err != nil {
				return fmt.Errorf("check if evm txn events are persisted: %s", err)
			}
			shouldPersistTxnHashEvents[e.TxHash] = areTxnEventsPersisted
		}
		if !shouldPersistTxnHashEvents[e.TxHash] {
			continue
		}

		blockHeader, ok := blockHeaderCache[e.BlockHash]
		if !ok {
			blockHeader, err = ef.ethClient.HeaderByNumber(ctx, big.NewInt(int64(e.BlockNumber)))
			if err != nil {
				return fmt.Errorf("get block header %d: %s", e.BlockNumber, err)
			}
			blockHeaderCache[e.BlockHash] = blockHeader
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
		tblEvents = append(tblEvents, tableland.EVMEvent{
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
			Timestamp: blockHeader.Time,
		})
	}

	if err := store.SaveEVMEvents(ctx, tblEvents); err != nil {
		return fmt.Errorf("persisting events: %s", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit db tx: %s", err)
	}

	return nil
}

// Based on https://github.com/json-iterator/go/issues/392
type OmitRawFieldExtension struct {
	jsoniter.DummyExtension
}

func (e *OmitRawFieldExtension) UpdateStructDescriptor(structDescriptor *jsoniter.StructDescriptor) {
	if binding := structDescriptor.GetField("Raw"); binding != nil {
		binding.ToNames = []string{}
	}
}
