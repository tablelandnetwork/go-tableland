package impl

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.uber.org/atomic"
)

// eventTypes are the event types that the event processor is interested to process
// and thus have execution logic for them.
var eventTypes = []eventfeed.EventType{
	eventfeed.RunSQL,
	eventfeed.CreateTable,
	eventfeed.SetController,
	eventfeed.TransferTable,
}

// EventProcessor processes new events detected by an event feed.
type EventProcessor struct {
	log      zerolog.Logger
	parser   parsing.SQLValidator
	executor executor.Executor
	ef       eventfeed.EventFeed
	config   *eventprocessor.Config
	chainID  tableland.ChainID

	nextHashCalcBlockNumber int64

	lock           sync.Mutex
	daemonCtx      context.Context
	daemonCancel   context.CancelFunc
	daemonCanceled chan struct{}

	// Metrics
	mBaseLabels                 []attribute.KeyValue
	mExecutionRound             atomic.Int64
	mLastProcessedHeight        atomic.Int64
	mBlockExecutionLatency      syncint64.Histogram
	mEventExecutionCounter      syncint64.Counter
	mTxnExecutionLatency        syncint64.Histogram
	mHashCalculationElapsedTime atomic.Int64
}

// New returns a new EventProcessor.
func New(
	parser parsing.SQLValidator,
	executor executor.Executor,
	ef eventfeed.EventFeed,
	chainID tableland.ChainID,
	opts ...eventprocessor.Option,
) (*EventProcessor, error) {
	log := logger.With().
		Str("component", "eventprocessor").
		Int64("chain_id", int64(chainID)).
		Logger()

	config := eventprocessor.DefaultConfig()
	for _, op := range opts {
		if err := op(config); err != nil {
			return nil, fmt.Errorf("applying option: %s", err)
		}
	}

	ep := &EventProcessor{
		log:      log,
		parser:   parser,
		executor: executor,
		ef:       ef,
		chainID:  chainID,
		config:   config,
	}
	if err := ep.initMetrics(chainID); err != nil {
		return nil, fmt.Errorf("initializing metric instruments: %s", err)
	}

	return ep, nil
}

// Start starts processing new events from the last processed height.
func (ep *EventProcessor) Start() error {
	ep.lock.Lock()
	defer ep.lock.Unlock()

	if ep.daemonCtx != nil {
		return fmt.Errorf("already started")
	}

	ep.log.Debug().Msg("starting daemon...")
	ctx, cls := context.WithCancel(context.Background())
	ep.daemonCtx = ctx
	ep.daemonCancel = cls
	ep.daemonCanceled = make(chan struct{})
	if err := ep.startDaemon(); err != nil {
		return fmt.Errorf("background daemon failed starting: %s", err)
	}
	ep.log.Info().Msg("started")

	return nil
}

// Stop stops processing new events.
func (ep *EventProcessor) Stop() {
	ep.lock.Lock()
	defer ep.lock.Unlock()
	if ep.daemonCtx == nil {
		return
	}

	ep.log.Debug().Msg("stopping syncer gracefully...")
	ep.daemonCancel()
	<-ep.daemonCanceled

	// Cleanup to allow StartSync() to be called again.
	ep.daemonCtx = nil
	ep.daemonCancel = nil
	ep.daemonCanceled = nil
	ep.mExecutionRound.Store(0)

	ep.log.Debug().Msg("syncer stopped")
}

func (ep *EventProcessor) startDaemon() error {
	// We start by fetching the lastest processed height to start processing
	// new events from that point forward.
	ctx, cls := context.WithTimeout(ep.daemonCtx, time.Second*10)
	defer cls()
	fromHeight, err := ep.executor.GetLastExecutedBlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("get last executed block number: %s", err)
	}
	ep.mLastProcessedHeight.Store(fromHeight)
	ep.nextHashCalcBlockNumber = nextMultipleOf(fromHeight, ep.config.HashCalcStep)

	// We fire an EventFeed asking for new events from the last processing height.
	// Notice that if the client calls StopSync(...) it will cancel fp.daemonCtx
	// which will cleanly close the EventFeed, and `defer close(ch)` making the processor
	// finish gracefully too.
	ch := make(chan eventfeed.BlockEvents, 500)
	go func() {
		defer close(ch)
		if err := ep.ef.Start(ep.daemonCtx, fromHeight+1, ch, eventTypes); err != nil {
			ep.log.Error().Err(err).Msg("query feed was closed unexpectedly")
			ep.Stop() // We cleanup daemon ctx and allow the processor to StartSync() cleanly if needed.
			return
		}
		ep.log.Info().Msg("event feed gracefully closed")
	}()

	// Listen to new events from the EventFeed, and process them.
	go func() {
		defer close(ep.daemonCanceled)
		defer ep.log.Info().Msg("processor gracefully closed")

		for bes := range ch {
			// If a runBlockEvents execution fails, we keep retrying since it *must* be
			// a transient error (e.g: the database is down, disk is corrupted, etc).
			// If the block has events that failed execution but are part of the protocol,
			// those won't make the block execution fail but only that query.
			// We should keep retrying because we *must* always be able to make progress.
			//
			// The validator operator should monitor the published metrics to detect if
			// we're continuously retrying which must signal something is definitely wrong with
			// our database, infrastructure, or there's a software bug.
			for {
				if ep.daemonCtx.Err() != nil {
					break
				}
				// fp.mExecutionRound is a value tracked by a metric that allows
				// to monitor if the current block execution is stuck.
				// Usually this value must be zero. Maybe 1 or 2 if
				// the database is temporarily down. Higher values indicate that we're
				// definitely stuck processing a block and definitely needs close attention.
				if err := ep.executeBlock(ep.daemonCtx, bes); err != nil {
					ep.log.Error().Int("attempt", int(ep.mExecutionRound.Load())).Err(err).Msg("executing block events")
					ep.mExecutionRound.Inc()
					time.Sleep(ep.config.BlockFailedExecutionBackoff)
					continue
				}
				break
			}
			ep.mExecutionRound.Store(0)
		}
	}()

	return nil
}

func (ep *EventProcessor) executeBlock(ctx context.Context, block eventfeed.BlockEvents) error {
	start := time.Now()
	bs, err := ep.executor.NewBlockScope(ctx, block.BlockNumber)
	if err != nil {
		return fmt.Errorf("opening block scope: %s", err)
	}
	defer func() {
		if err := bs.Close(); err != nil {
			ep.log.Error().Err(err).Msg("closing block scope")
		}
	}()

	if block.BlockNumber >= ep.nextHashCalcBlockNumber {
		if _, err := ep.calculateHash(ctx, bs); err != nil {
			return fmt.Errorf("calculate hash: %s", err)
		}
		ep.nextHashCalcBlockNumber = nextMultipleOf(block.BlockNumber, ep.config.HashCalcStep)
	}

	receipts := make([]eventprocessor.Receipt, 0, len(block.Txns))
	for idxInBlock, txnEvents := range block.Txns {
		if ep.config.DedupExecutedTxns {
			ok, err := bs.TxnReceiptExists(ctx, txnEvents.TxnHash)
			if err != nil {
				return fmt.Errorf("checking if receipt already exist: %s", err)
			}
			if ok {
				ep.log.Info().
					Str("txn_hash", txnEvents.TxnHash.Hex()).
					Msg("skipping execution since was already processed due to a reorg")
				continue
			}
		}

		start := time.Now()
		txnExecResult, err := bs.ExecuteTxnEvents(ctx, txnEvents)
		if err != nil {
			return fmt.Errorf("executing txn events: %s", err)
		}
		receipt := eventprocessor.Receipt{
			ChainID:      ep.chainID,
			BlockNumber:  block.BlockNumber,
			IndexInBlock: int64(idxInBlock),
			TxnHash:      txnEvents.TxnHash.Hex(),

			TableID:       txnExecResult.TableID,
			Error:         txnExecResult.Error,
			ErrorEventIdx: txnExecResult.ErrorEventIdx,
		}
		receipts = append(receipts, receipt)

		if receipt.Error != nil {
			// Some acceptable failure happened (e.g: invalid syntax, inserting
			// a string in an integer column, etc). Just log it, and move on.
			ep.log.Info().Str("fail_cause", *receipt.Error).Msg("event execution failed")
		}

		for _, e := range txnEvents.Events {
			attrs := append([]attribute.KeyValue{
				attribute.String("eventtype", reflect.TypeOf(e).String()),
			}, ep.mBaseLabels...)
			ep.mEventExecutionCounter.Add(ctx, 1, attrs...)
		}
		ep.mTxnExecutionLatency.Record(ctx, time.Since(start).Milliseconds(), ep.mBaseLabels...)
	}
	// Save receipts.
	if err := bs.SaveTxnReceipts(ctx, receipts); err != nil {
		return fmt.Errorf("saving txn receipts: %s", err)
	}
	ep.log.Debug().Int64("height", block.BlockNumber).Int("receipts", len(receipts)).Msg("saved receipts")

	// Update the last processed height.
	if err := bs.SetLastProcessedHeight(ctx, block.BlockNumber); err != nil {
		return fmt.Errorf("set new processed height %d: %s", block.BlockNumber, err)
	}

	if err := bs.Commit(); err != nil {
		return fmt.Errorf("committing changes: %s", err)
	}
	ep.log.Debug().
		Int64("height", block.BlockNumber).
		Int64("exec_ms", time.Since(start).Milliseconds()).
		Msg("new last processed height")

	ep.mLastProcessedHeight.Store(block.BlockNumber)
	ep.mBlockExecutionLatency.Record(ctx, time.Since(start).Milliseconds(), ep.mBaseLabels...)

	return nil
}

func (ep *EventProcessor) calculateHash(ctx context.Context, bs executor.BlockScope) (string, error) {
	startTime := time.Now()
	stateHash, err := bs.StateHash(ctx, ep.chainID)
	if err != nil {
		return "", fmt.Errorf("calculating hash for current block: %s", err)
	}
	elapsedTime := time.Since(startTime).Milliseconds()
	ep.log.Info().
		Str("hash", stateHash.Hash()).
		Int64("block_number", stateHash.BlockNumber()).
		Int64("chain_id", stateHash.ChainID()).
		Int64("elapsed_time", elapsedTime).
		Msg("state hash")

	ep.mHashCalculationElapsedTime.Store(elapsedTime)

	if err := telemetry.Collect(ctx, stateHash); err != nil {
		return "", fmt.Errorf("calculating hash for current block: %s", err)
	}

	return stateHash.Hash(), nil
}

func nextMultipleOf(x, y int64) int64 {
	return y * ((x + y) / y)
}
