package eventprocessor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/txn"
)

var log = logger.With().Str("component", "eventprocessor").Logger()

type EventProcessor struct {
	parser parsing.SQLValidator
	txnp   txn.TxnProcessor
	qf     eventfeed.EventFeed

	config         *eventprocessor.Config
	lock           sync.Mutex
	daemonCtx      context.Context
	daemonCancel   context.CancelFunc
	daemonCanceled chan struct{}
}

func New(parser parsing.SQLValidator,
	txnp txn.TxnProcessor,
	qf eventfeed.EventFeed,
	opts ...eventprocessor.Option) (*EventProcessor, error) {
	config := eventprocessor.DefaultConfig()
	for _, op := range opts {
		if err := op(config); err != nil {
			return nil, fmt.Errorf("applying option: %s", err)
		}
	}
	return &EventProcessor{
		parser: parser,
		txnp:   txnp,
		qf:     qf,
		config: config,
	}, nil
}

func (ep *EventProcessor) StartSync() error {
	ep.lock.Lock()
	defer ep.lock.Unlock()
	if ep.daemonCtx != nil {
		return fmt.Errorf("processor already started")
	}

	ctx, cls := context.WithCancel(context.Background())
	ep.daemonCtx = ctx
	ep.daemonCancel = cls
	ep.daemonCanceled = make(chan struct{})
	if err := ep.startDaemon(); err != nil {
		return fmt.Errorf("background daemon failed starting: %s", err)
	}

	log.Info().Msg("syncer started")
	return nil
}

func (ep *EventProcessor) StopSync() {
	ep.lock.Lock()
	defer ep.lock.Unlock()
	if ep.daemonCtx == nil {
		return
	}

	log.Debug().Msg("stopping syncer gracefully...")
	ep.daemonCancel()
	<-ep.daemonCanceled

	ep.daemonCtx = nil
	ep.daemonCancel = nil
	ep.daemonCanceled = nil

	log.Debug().Msg("syncer stopped")
}

func (fp *EventProcessor) startDaemon() error {
	log.Debug().Msg("starting daemon")

	ctx, cls := context.WithTimeout(fp.daemonCtx, time.Second*10)
	defer cls()
	b, err := fp.txnp.OpenBatch(ctx)
	if err != nil {
		return fmt.Errorf("opening batch in daemon: %s", err)
	}
	fromHeight, err := b.GetLastProcessedHeight(ctx)
	if err != nil {
		log.Err(err).Msg("getting last processed height")
	}
	if err := b.Close(ctx); err != nil {
		return fmt.Errorf("closing batch: %s", err)
	}

	ch := make(chan eventfeed.BlockEvents)
	go func() {
		defer close(ch)
		if err := fp.qf.Start(fp.daemonCtx, int64(fromHeight), ch, []eventfeed.EventType{eventfeed.RunSQL}); err != nil {
			log.Error().Err(err).Msg("query feed was closed unexpectedly")
			fp.StopSync()
			return
		}
		log.Info().Msg("query feed gracefully closed")
	}()

	go func() {
		defer close(fp.daemonCanceled)
		for {
			select {
			case bqs, ok := <-ch:
				if !ok {
					log.Info().Msg("background daemon closed")
					return
				}

				// If a runBlockQueries execution fails, we keep retrying since it *must* be
				// a transient error (e.g: the database is down, disk is corrupted, etc).
				// If the block has queries that failed execution but are part of the protocol,
				// those won't make the block execution fail but only that query.
				// We should keep retrying because we *must* always be able to make progress.
				//
				// The validator operator should monitor the published metrics to detect if
				// we're continously retrying which must signal something is definitely wrong with
				// our database, infrastructure, or there's a software bug.
				for {
					var attempt int
					if err := fp.runBlockQueries(fp.daemonCtx, bqs); err != nil {
						log.Error().Int("attempt", attempt).Err(err).Msg("executing block queries")
						attempt++
						time.Sleep(fp.config.BlockFailedExecutionBackoff)
						continue
					}
					break
				}
			}
		}
	}()

	return nil
}

func (fp *EventProcessor) runBlockQueries(ctx context.Context, bqs eventfeed.BlockEvents) error {
	b, err := fp.txnp.OpenBatch(ctx)
	if err != nil {
		return fmt.Errorf("opening batch: %s", err)
	}
	defer func() {
		if err := b.Close(ctx); err != nil {
			log.Error().Err(err).Msg("closing batch")
		}
	}()

	// Get last processed height.
	lastHeight, err := b.GetLastProcessedHeight(ctx)
	if err != nil {
		return fmt.Errorf("get last processed height: %s", err)
	}

	// The new height to process must be strictly greated than the last processed height.
	if lastHeight >= bqs.BlockNumber {
		return fmt.Errorf("last processed height %d isn't smaller than new height %d", lastHeight, bqs.BlockNumber)
	}

	// Execute each query event and track the execution trace.
	//traces := make([]txn.TxnExecutionTrace, len(bqs.Queries))
	for _, e := range bqs.Events {
		/* TODO(jsign)
		traces[i] = txn.TxnExecutionTrace{
			BlockNumber: bqs.BlockNumber,
			Query:       q,
			Error:       fp.executeQuery(ctx, b, q),
		}
		*/
		if err := fp.executeEvent(ctx, b, e); err != nil {
			log.Warn().Err(err).Msg("executing query")
		}

	}

	/* TODO(jsign)
	// Persist the execution trace for this block height. This is done for
	// debuggability, history tracking, and potentially future state-comparison between validators.
	if err := fp.txnp.SaveBlockQueriesTrace(ctx, bqs); err != nil {
		return fmt.Errorf("saving block queries: %s", err)
	}
	*/

	// Update the last processed height.
	if err := b.SetLastProcessedHeight(ctx, bqs.BlockNumber); err != nil {
		return fmt.Errorf("set new processed height %d: %s", bqs.BlockNumber, err)
	}

	if err := b.Commit(ctx); err != nil {
		return fmt.Errorf("committing changes: %s", err)
	}

	// TODO(jsign): metric counter last processed commited height with error/no-error
	// TODO(jsign): metric latency of block processing.

	return nil
}

func (fp *EventProcessor) executeEvent(ctx context.Context, b txn.Batch, e interface{}) error {
	switch e := e.(type) {
	case *ethereum.ContractRunSQL:
		readStmt, mutatingStmts, err := fp.parser.ValidateRunSQL(e.Statement)
		if err != nil {
			return fmt.Errorf("validating query: %s", err)
		}
		if readStmt != nil {
			return errors.New("query is a read statement")
		}
		if err := b.ExecWriteQueries(ctx, mutatingStmts); err != nil {
			return fmt.Errorf("executing mutating-query: %s", err)
		}
	default:
		return fmt.Errorf("unkown event type %t", e)
	}

	return nil
}
