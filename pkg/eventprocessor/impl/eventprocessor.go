package impl

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/txn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.uber.org/atomic"
)

var (
	// eventTypes are the event types that the event processor is interested to process
	// and thus have execution logic for them.
	eventTypes = []eventfeed.EventType{eventfeed.RunSQL, eventfeed.CreateTable, eventfeed.SetController}
)

// EventProcessor processes new events detected by an event feed.
type EventProcessor struct {
	log     zerolog.Logger
	parser  parsing.SQLValidator
	txnp    txn.TxnProcessor
	ef      eventfeed.EventFeed
	config  *eventprocessor.Config
	chainID tableland.ChainID

	lock           sync.Mutex
	daemonCtx      context.Context
	daemonCancel   context.CancelFunc
	daemonCanceled chan struct{}

	// Metrics
	mBaseLabels            []attribute.KeyValue
	mExecutionRound        atomic.Int64
	mLastProcessedHeight   atomic.Int64
	mBlockExecutionLatency syncint64.Histogram
	mEventExecutionCounter syncint64.Counter
	mEventExecutionLatency syncint64.Histogram
}

// New returns a new EventProcessor.
func New(
	parser parsing.SQLValidator,
	txnp txn.TxnProcessor,
	ef eventfeed.EventFeed,
	chainID tableland.ChainID,
	opts ...eventprocessor.Option) (*EventProcessor, error) {
	config := eventprocessor.DefaultConfig()
	for _, op := range opts {
		if err := op(config); err != nil {
			return nil, fmt.Errorf("applying option: %s", err)
		}
	}

	log := logger.With().
		Str("component", "eventprocessor").
		Int64("chainID", int64(chainID)).
		Logger()
	ep := &EventProcessor{
		log:     log,
		parser:  parser,
		txnp:    txnp,
		ef:      ef,
		chainID: chainID,
		config:  config,
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
	b, err := ep.txnp.OpenBatch(ctx)
	if err != nil {
		return fmt.Errorf("opening batch in daemon: %s", err)
	}
	fromHeight, err := b.GetLastProcessedHeight(ctx)
	if err != nil {
		ep.log.Err(err).Msg("getting last processed height")
	}
	if err := b.Close(ctx); err != nil {
		return fmt.Errorf("closing batch: %s", err)
	}
	ep.mLastProcessedHeight.Store(fromHeight)

	// We fire an EventFeed asking for new events from the last processing height.
	// Notice that if the client calls StopSync(...) it will cancel fp.daemonCtx
	// which will cleanly close the EventFeed, and `defer close(ch)` making the processor
	// finish gracefully too.
	ch := make(chan eventfeed.BlockEvents)
	go func() {
		defer close(ch)
		if err := ep.ef.Start(ep.daemonCtx, fromHeight, ch, eventTypes); err != nil {
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

		for bqs := range ch {
			// If a runBlockQueries execution fails, we keep retrying since it *must* be
			// a transient error (e.g: the database is down, disk is corrupted, etc).
			// If the block has queries that failed execution but are part of the protocol,
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
				if err := ep.runBlockQueries(ep.daemonCtx, bqs); err != nil {
					ep.log.Error().Int("attempt", int(ep.mExecutionRound.Load())).Err(err).Msg("executing block queries")
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

func (ep *EventProcessor) runBlockQueries(ctx context.Context, bqs eventfeed.BlockEvents) error {
	start := time.Now()
	b, err := ep.txnp.OpenBatch(ctx)
	if err != nil {
		return fmt.Errorf("opening batch: %s", err)
	}
	defer func() {
		if err := b.Close(ctx); err != nil {
			ep.log.Error().Err(err).Msg("closing batch")
		}
	}()

	// Get last processed height.
	lastHeight, err := b.GetLastProcessedHeight(ctx)
	if err != nil {
		return fmt.Errorf("get last processed height: %s", err)
	}

	// The new height to process must be strictly greater than the last processed height.
	if lastHeight >= bqs.BlockNumber {
		return fmt.Errorf("last processed height %d isn't smaller than new height %d", lastHeight, bqs.BlockNumber)
	}

	receipts := make([]eventprocessor.Receipt, len(bqs.Events))
	for i, e := range bqs.Events {
		start := time.Now()
		receipt, err := ep.executeEvent(ctx, b, bqs.BlockNumber, e)
		if err != nil {
			// Some retriable error happened, abort the block execution
			// and retry later.
			return fmt.Errorf("executing query: %s", err)
		}
		receipts[i] = receipt
		attrs := append([]attribute.KeyValue{
			attribute.String("eventtype", fmt.Sprintf("%T", e)),
		}, ep.mBaseLabels...)
		if receipt.Error != nil {
			// Some acceptable failure happened (e.g: invalid syntax, inserting
			// a string in an integer column, etc). Just log it, and move on.
			ep.log.Info().Str("failCause", *receipt.Error).Msg("event execution failed")
			attrs = append(attrs, attribute.String("failCause", *receipt.Error))
		}

		ep.mEventExecutionCounter.Add(ctx, 1, attrs...)
		ep.mEventExecutionLatency.Record(ctx, time.Since(start).Milliseconds(), attrs...)
	}
	// Save receipts.
	if err := b.SaveTxnReceipts(ctx, receipts); err != nil {
		return fmt.Errorf("saving txn receipts: %s", err)
	}
	ep.log.Debug().Int64("height", bqs.BlockNumber).Int("receipts", len(receipts)).Msg("saved receipts")

	// Update the last processed height.
	if err := b.SetLastProcessedHeight(ctx, bqs.BlockNumber); err != nil {
		return fmt.Errorf("set new processed height %d: %s", bqs.BlockNumber, err)
	}

	if err := b.Commit(ctx); err != nil {
		return fmt.Errorf("committing changes: %s", err)
	}
	ep.log.Debug().Int64("height", bqs.BlockNumber).Msg("new last processed height")

	ep.mLastProcessedHeight.Store(bqs.BlockNumber)
	ep.mBlockExecutionLatency.Record(ctx, time.Since(start).Milliseconds(), ep.mBaseLabels...)

	return nil
}

// executeEvent executes an event. If the event execution was successful, it returns "", nil.
// If the event execution:
// 1) Has an acceptable execution failure, it returns the failure cause in the first return parameter,
//    and nil in the second one.
// 2) Has an unknown infrastructure error, then it returns ("", err) where err is the underlying error.
//    Probably the caller will want to retry executing this event later when this problem is solved and
//    retry the event.
func (ep *EventProcessor) executeEvent(
	ctx context.Context,
	b txn.Batch,
	blockNumber int64,
	be eventfeed.BlockEvent) (eventprocessor.Receipt, error) {
	switch e := be.Event.(type) {
	case *ethereum.ContractRunSQL:
		ep.log.Debug().Str("statement", e.Statement).Msgf("executing run-sql event")
		receipt, err := ep.executeRunSQLEvent(ctx, b, blockNumber, be, e)
		if err != nil {
			return eventprocessor.Receipt{}, fmt.Errorf("executing runsql event: %s", err)
		}
		return receipt, nil
	case *ethereum.ContractCreateTable:
		ep.log.Debug().
			Str("caller", e.Caller.Hex()).
			Str("tokenId", e.TableId.String()).
			Str("statement", e.Statement).
			Msgf("executing create-table event")
		receipt, err := ep.executeCreateTableEvent(ctx, b, blockNumber, be, e)
		if err != nil {
			return eventprocessor.Receipt{}, fmt.Errorf("executing create-table event: %s", err)
		}
		return receipt, nil
	case *ethereum.ContractSetController:
		ep.log.Debug().
			Str("controller", e.Controller.Hex()).
			Str("tokenId", e.TableId.String()).
			Msgf("executing set-controller event")
		receipt, err := ep.executeSetControllerEvent(ctx, b, blockNumber, be, e)
		if err != nil {
			return eventprocessor.Receipt{}, fmt.Errorf("executing set-controller event: %s", err)
		}
		return receipt, nil
	default:
		return eventprocessor.Receipt{}, fmt.Errorf("unknown event type %t", e)
	}
}

func (ep *EventProcessor) executeCreateTableEvent(
	ctx context.Context,
	b txn.Batch,
	blockNumber int64,
	be eventfeed.BlockEvent,
	e *ethereum.ContractCreateTable) (eventprocessor.Receipt, error) {
	receipt := eventprocessor.Receipt{
		ChainID:     ep.chainID,
		BlockNumber: blockNumber,
		TxnHash:     be.TxnHash.String(),
	}
	createStmt, err := ep.parser.ValidateCreateTable(e.Statement)
	if err != nil {
		err := fmt.Sprintf("query validation: %s", err)
		receipt.Error = &err
		return receipt, nil
	}

	if e.TableId == nil {
		err := "token id is empty"
		receipt.Error = &err
		return receipt, nil
	}
	tableID := tableland.TableID(*e.TableId)

	if err := b.InsertTable(ctx, tableID, e.Caller.Hex(), createStmt); err != nil {
		var pgErr *txn.ErrQueryExecution
		if errors.As(err, &pgErr) {
			err := fmt.Sprintf("table creation execution failed (code: %s, msg: %s)", pgErr.Code, pgErr.Msg)
			receipt.Error = &err
			return receipt, nil
		}
		return eventprocessor.Receipt{}, fmt.Errorf("executing table creation: %s", err)
	}
	receipt.TableID = &tableID

	return receipt, nil
}

func (ep *EventProcessor) executeRunSQLEvent(
	ctx context.Context,
	b txn.Batch,
	blockNumber int64,
	be eventfeed.BlockEvent,
	e *ethereum.ContractRunSQL) (eventprocessor.Receipt, error) {
	receipt := eventprocessor.Receipt{
		ChainID:     ep.chainID,
		BlockNumber: blockNumber,
		TxnHash:     be.TxnHash.String(),
	}
	readStmt, mutatingStmts, err := ep.parser.ValidateRunSQL(e.Statement)
	if err != nil {
		err := fmt.Sprintf("parsing query: %s", err)
		receipt.Error = &err
		return receipt, nil
	}
	if readStmt != nil {
		err := "this is a read query, skipping"
		receipt.Error = &err
		return receipt, nil
	}
	tableID := tableland.TableID(*e.TableId)
	targetedTableID := mutatingStmts[0].GetTableID()
	if targetedTableID.ToBigInt().Cmp(tableID.ToBigInt()) != 0 {
		err := fmt.Sprintf("query targets table id %s and not %s", targetedTableID, tableID)
		receipt.Error = &err
		return receipt, nil
	}
	if err := b.ExecWriteQueries(ctx, e.Caller, mutatingStmts, e.IsOwner, &policy{e.Policy}); err != nil {
		var pgErr *txn.ErrQueryExecution
		if errors.As(err, &pgErr) {
			err := fmt.Sprintf("db query execution failed (code: %s, msg: %s)", pgErr.Code, pgErr.Msg)
			receipt.Error = &err
			return receipt, nil
		}
		return eventprocessor.Receipt{}, fmt.Errorf("executing mutating-query: %s", err)
	}
	tblID := mutatingStmts[0].GetTableID()
	receipt.TableID = &tblID

	return receipt, nil
}

func (ep *EventProcessor) executeSetControllerEvent(
	ctx context.Context,
	b txn.Batch,
	blockNumber int64,
	be eventfeed.BlockEvent,
	e *ethereum.ContractSetController) (eventprocessor.Receipt, error) {
	receipt := eventprocessor.Receipt{
		ChainID:     ep.chainID,
		BlockNumber: blockNumber,
		TxnHash:     be.TxnHash.String(),
	}

	if e.TableId == nil {
		err := "token id is empty"
		receipt.Error = &err
		return receipt, nil
	}
	tableID := tableland.TableID(*e.TableId)

	if err := b.SetController(ctx, tableID, e.Controller); err != nil {
		var pgErr *txn.ErrQueryExecution
		if errors.As(err, &pgErr) {
			err := fmt.Sprintf("set controller execution failed (code: %s, msg: %s)", pgErr.Code, pgErr.Msg)
			receipt.Error = &err
			return receipt, nil
		}
		return eventprocessor.Receipt{}, fmt.Errorf("executing set controller: %s", err)
	}

	receipt.TableID = &tableID

	return receipt, nil
}

type policy struct {
	ethereum.TablelandControllerLibraryPolicy
}

func (p *policy) IsInsertAllowed() bool {
	return p.TablelandControllerLibraryPolicy.AllowInsert
}

func (p *policy) IsUpdateAllowed() bool {
	return p.TablelandControllerLibraryPolicy.AllowUpdate
}

func (p *policy) IsDeleteAllowed() bool {
	return p.TablelandControllerLibraryPolicy.AllowDelete
}

func (p *policy) WhereClause() string {
	return p.TablelandControllerLibraryPolicy.WhereClause
}

func (p *policy) UpdateColumns() []string {
	return p.TablelandControllerLibraryPolicy.UpdatableColumns
}
