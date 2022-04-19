package impl

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	tbleth "github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.uber.org/atomic"
)

var log = logger.With().Str("component", "eventfeed").Logger()

// EventFeed provides a stream of filtered events from a SC.
type EventFeed struct {
	ethClient eventfeed.ChainClient
	scAddress common.Address
	scABI     *abi.ABI
	config    *eventfeed.Config

	// Metrics
	mEventTypeCounter syncint64.Counter
	mCurrentHeight    atomic.Int64
}

// New returns a new EventFeed.
func New(ethClient eventfeed.ChainClient, scAddress common.Address, opts ...eventfeed.Option) (*EventFeed, error) {
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
	ef := &EventFeed{
		ethClient: ethClient,
		scAddress: scAddress,
		scABI:     scABI,
		config:    config,
	}
	if err := ef.initMetrics(); err != nil {
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
	log.Debug().Msg("starting...")
	defer log.Debug().Msg("stopped")

	// Spinup a background process that will post to chHeads when a new block is detected.
	// This channel will be the heart-beat to pull new logs from the chain.
	// We defer the ctx cancelation to be sure we always gracefully close this background go routine
	// in any event that returns this function.
	ctx, cls := context.WithCancel(ctx)
	defer cls()
	chHeads := make(chan *types.Header, 1)
	if err := ef.notifyNewHeads(ctx, chHeads); err != nil {
		return fmt.Errorf("creating background head notificator: %s", err)
	}

	// Create filterTopics that will be used to only listening for the desired events.
	filterTopics, err := ef.getTopicsForEventTypes(filterEventTypes)
	if err != nil {
		return fmt.Errorf("creating topics for filtered event types: %s", err)
	}

	// Listen for new blocks, and get new events.
	for h := range chHeads {
		log.Debug().Int64("height", h.Number.Int64()).Msg("received new chain header")

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

			if toHeight-fromHeight+1 > int64(ef.config.MaxBlocksFetchSize) {
				toHeight = fromHeight + int64(ef.config.MaxBlocksFetchSize) - 1
			}

			// Ask for the desired events between fromHeight to toHeight.
			query := ethereum.FilterQuery{
				FromBlock: big.NewInt(fromHeight),
				ToBlock:   big.NewInt(toHeight),
				Addresses: []common.Address{ef.scAddress},
				Topics:    [][]common.Hash{filterTopics},
			}
			log.Debug().Int64("from", fromHeight).Int64("to", toHeight).Msg("calling filter logs")
			logs, err := ef.ethClient.FilterLogs(ctx, query)
			if err != nil {
				// If we got an error here, log it but allow to be retried
				// in the next head. Probably the API can have transient unavailability.
				log.Warn().Err(err).Msgf("filter logs from %d to %d", fromHeight, toHeight)
				time.Sleep(ef.config.ChainAPIBackoff)
				continue
			}

			// If there're no events, nothing to do here.
			if len(logs) == 0 {
				log.Debug().Msg("no filter logs")
			} else {
				// We received new events. We'll group/pack them by block number in
				// BlockEvents structs, and send them to the `ch` channel provided
				// by the caller.
				bq := eventfeed.BlockEvents{
					BlockNumber: int64(logs[0].BlockNumber),
				}
				for _, l := range logs {
					if bq.BlockNumber != int64(l.BlockNumber) {
						ch <- bq
						bq = eventfeed.BlockEvents{
							BlockNumber: int64(l.BlockNumber),
						}
					}

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
func (ef *EventFeed) parseEvent(l types.Log) (interface{}, error) {
	// We get an event descriptior from the common.Hash value that is always
	// in Topic[0] in events. This is an ID for the kind of event.
	eventDescr, err := ef.scABI.EventByID(l.Topics[0])
	if err != nil {
		return nil, fmt.Errorf("detecting event type: %s", err)
	}

	se, ok := eventfeed.SupportedEvents[eventfeed.EventType(eventDescr.Name)]
	if !ok {
		return nil, fmt.Errorf("unknown event type %s", eventDescr.Name)
	}
	// Create a new *ContractXXXX struct that corresponds to this event.
	// e.g: *ContractRunSQL if this event was one fired by runSQL(..) SC function.
	i := reflect.New(se).Interface()

	// Now we unmarshal the event data, to the *ContractXXX struct.
	// First, we unmarshal the information contained in the `data` of the event, which
	// are non-indexed fields of the event.
	if len(l.Data) > 0 {
		if err := ef.scABI.UnpackIntoInterface(i, eventDescr.Name, l.Data); err != nil {
			return nil, fmt.Errorf("unpacking into interface: %s", err)
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
		return nil, fmt.Errorf("unpacking indexed topics: %s", err)
	}
	// Note that the above two steps of unmarshalling isn't something particular
	// to us, it's just how Ethereum works.
	// This parsedEvent(...) function was coded in a generic way, so it will hardly
	// ever change.

	ef.mEventTypeCounter.Add(context.Background(), 1, attribute.String("name", eventDescr.Name))

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

// notifyNewHeads will send to the provided channel new detected heads in the chain.
// It's mandatory that the caller cancels the provided context to gracefully close the background process.
// When this happens the provided channel will be closed.
func (ef *EventFeed) notifyNewHeads(ctx context.Context, ch chan *types.Header) error {
	h, err := ef.ethClient.HeaderByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("get current head: %s", err)
	}
	ch <- h

	subHeader, err := ef.ethClient.SubscribeNewHead(ctx, ch)
	if err != nil {
		return fmt.Errorf("subscribing to new heads: %s", err)
	}
	go func() {
		defer close(ch)
		defer subHeader.Unsubscribe()
		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("gracefully closing new heads subscription")
				return
			case err := <-subHeader.Err():
				log.Error().Err(err).Msg("new heads subscription")
				return
			}
		}
	}()
	return nil
}
