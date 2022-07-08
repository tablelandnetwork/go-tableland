package impl

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
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
	chainID tableland.ChainID,
	ethClient eventfeed.ChainClient,
	scAddress common.Address,
	opts ...eventfeed.Option) (*EventFeed, error) {
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
		Int64("chainID", int64(chainID)).
		Logger()
	ef := &EventFeed{
		log:                log,
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
	filterEventTypes []eventfeed.EventType) error {
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
				Int64("maxBlocksFetchSize", int64(ef.maxBlocksFetchSize)).
				Msg("received new chain header")
		}
		// We do a for loop since we'll try to catch from fromHeight to the new reported
		// head in batches with max size MaxEventsBatchSize. This is important to
		// avoid asking the API for very big ranges (e.g: newHead - fromHeight > 100k) since
		// that could would be too big (i.e: make the API fail, the response would be too big,
		// and would consume too much memory).
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
				if strings.Contains(err.Error(), "read limit exceeded") {
					ef.maxBlocksFetchSize = ef.maxBlocksFetchSize * 80 / 100
				} else {
					time.Sleep(ef.config.ChainAPIBackoff)
				}
				continue
			}

			if len(logs) > 0 {
				// We received new events. We'll group/pack them by block number in
				// BlockEvents structs, and send them to the `ch` channel provided
				// by the caller.
				bq := eventfeed.BlockEvents{
					BlockNumber: int64(logs[0].BlockNumber),
				}
				observedTxns := map[string]struct{}{}
				for _, l := range logs {
					if bq.BlockNumber != int64(l.BlockNumber) {
						ch <- bq
						bq = eventfeed.BlockEvents{
							BlockNumber: int64(l.BlockNumber),
						}
						observedTxns = map[string]struct{}{}
					}
					if _, ok := observedTxns[l.TxHash.Hex()]; ok {
						ef.log.Warn().Str("txnHash", l.TxHash.String()).Msg("txn has more than one event")
						continue
					}
					observedTxns[l.TxHash.Hex()] = struct{}{}

					event, err := ef.parseEvent(l)
					if err != nil {
						return fmt.Errorf("couldn't parse event: %s", err)
					}
					bq.Events = append(bq.Events, event)
				}
				// Sent last block events construction of the loop.
				ch <- bq
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

// parseEvent deconstructs a raw event that was received from the Ethereum node,
// to a structured representation. Since the event can be from different types,
// we return an interface.
// Every possible type in the interface{} is an auto-generated struct by
// `make ethereum` named `Contract*` (e.g: ContractRunSQL, ContractTransfer, etc).
// See this mapping in the `supportedEvents` map global variable in this file.
func (ef *EventFeed) parseEvent(l types.Log) (eventfeed.BlockEvent, error) {
	// We get an event descriptior from the common.Hash value that is always
	// in Topic[0] in events. This is an ID for the kind of event.
	eventDescr, err := ef.scABI.EventByID(l.Topics[0])
	if err != nil {
		return eventfeed.BlockEvent{}, fmt.Errorf("detecting event type: %s", err)
	}

	se, ok := eventfeed.SupportedEvents[eventfeed.EventType(eventDescr.Name)]
	if !ok {
		return eventfeed.BlockEvent{}, fmt.Errorf("unknown event type %s", eventDescr.Name)
	}
	// Create a new *ContractXXXX struct that corresponds to this event.
	// e.g: *ContractRunSQL if this event was one fired by runSQL(..) SC function.
	i := reflect.New(se).Interface()

	// Now we unmarshal the event data, to the *ContractXXX struct.
	// First, we unmarshal the information contained in the `data` of the event, which
	// are non-indexed fields of the event.
	if len(l.Data) > 0 {
		if err := ef.scABI.UnpackIntoInterface(i, eventDescr.Name, l.Data); err != nil {
			return eventfeed.BlockEvent{}, fmt.Errorf("unpacking into interface: %s", err)
		}
	}
	// Second, we unmarshal indexed fields which aren't in data but in Topics[:1].
	var indexed abi.Arguments
	for _, arg := range eventDescr.Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	if err := abi.ParseTopics(i, indexed, l.Topics[1:]); err != nil {
		return eventfeed.BlockEvent{}, fmt.Errorf("unpacking indexed topics: %s", err)
	}
	// Note that the above two steps of unmarshalling isn't something particular
	// to us, it's just how Ethereum works.
	// This parsedEvent(...) function was coded in a generic way, so it will hardly
	// ever change.

	attrs := append([]attribute.KeyValue{attribute.String("name", eventDescr.Name)}, ef.mBaseLabels...)
	ef.mEventTypeCounter.Add(context.Background(), 1, attrs...)

	return eventfeed.BlockEvent{TxnHash: l.TxHash, Event: i}, nil
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
				clientCh <- h
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
