package feedprocessor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/queryfeed"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/txn"
)

type FeedProcessor struct {
	parser parsing.SQLValidator
	txnp   txn.TxnProcessor
	qf     queryfeed.QueryFeed

	lock           sync.Mutex
	daemonCtx      context.Context
	daemonCancel   context.CancelFunc
	daemonCanceled chan struct{}
}

func New(parser parsing.SQLValidator, txnp txn.TxnProcessor, qf queryfeed.QueryFeed) *FeedProcessor {
	return &FeedProcessor{
		parser: parser,
		txnp:   txnp,
		qf:     qf,
	}
}

func (fp *FeedProcessor) StartSync() error {
	fp.lock.Lock()
	defer fp.lock.Unlock()
	if fp.daemonCtx != nil {
		return fmt.Errorf("processor already started")
	}

	ctx, cls := context.WithCancel(context.Background())
	fp.daemonCtx = ctx
	fp.daemonCancel = cls
	fp.daemonCanceled = make(chan struct{})
	go fp.daemon()

	return nil
}

func (fp *FeedProcessor) StopSync() {
	fp.lock.Lock()
	defer fp.lock.Unlock()
	if fp.daemonCtx == nil {
		return
	}

	log.Debug().Msg("stopping feed processor")
	fp.daemonCancel()
	<-fp.daemonCanceled

	fp.daemonCtx = nil
	fp.daemonCancel = nil
	fp.daemonCanceled = nil

	log.Debug().Msg("feed processor stopped")
}

func (fp *FeedProcessor) daemon() {
	log.Debug().Msg("starting feed processor daemon")

	ch := make(chan queryfeed.BlockEvents)

	b, err := fp.txnp.OpenBatch(fp.daemonCtx)
	if err != nil {
		log.Error().Msgf("opening batch in daemon: %s", err)
		return
	}

	ctx, cls := context.WithTimeout(fp.daemonCtx, time.Second*5)
	defer cls()
	fromHeight, err := b.GetLastProcessedHeight(ctx)
	if err != nil {
		log.Err(err).Msg("getting last processed height")
	}
	ctx, cls = context.WithTimeout(fp.daemonCtx, time.Second*5)
	defer cls()

	if err := b.Close(fp.daemonCtx); err != nil {
		log.Err(err).Msg("closing batch")
		return
	}

	go func() {
		defer close(ch)
		if err := fp.qf.Start(fp.daemonCtx, int64(fromHeight), ch); err != nil {
			fp.StopSync()
			return
		}
		log.Info().Msg("closing feed processor daemon")
	}()

	for {
		select {
		case bqs, ok := <-ch:
			if !ok {
				return
			}
			if err := fp.runBlockQueries(fp.daemonCtx, bqs); err != nil {
			}
		}
	}
}

func (fp *FeedProcessor) runBlockQueries(ctx context.Context, bqs queryfeed.BlockEvents) error {
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

	// TODO(jsign)
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

	// TODO(jsign): metrics in general.
	//t.incrementRunSQLCount(ctx, ctrl)

	return nil
}

func (fp *FeedProcessor) executeEvent(ctx context.Context, b txn.Batch, e interface{}) error {
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
