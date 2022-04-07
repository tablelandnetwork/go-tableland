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
	"github.com/textileio/go-tableland/pkg/txn"
)

type FeedProcessor struct {
	parser parsing.SQLValidator
	txnp   txn.TxnProcessor
	qf     queryfeed.QueryFeed

	lock         sync.Mutex
	daemonCtx    context.Context
	daemonCancel context.CancelFunc
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
	go fp.daemon()

	return nil
}

func (fp *FeedProcessor) StopSync() error {
	fp.lock.Lock()
	defer fp.lock.Unlock()
	if fp.daemonCtx == nil {
		return fmt.Errorf("processor isn't running")
	}

	panic("TODO(jsign): do actual logic")

	fp.daemonCtx = nil
	fp.daemonCancel = nil

	return nil
}

func (fp *FeedProcessor) daemon() {
	log.Debug().Msg("starting feed processor daemon")

	ch := make(chan interface{})

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

	feedCtx, feedCls := context.WithCancel(context.Background())
	go func() {
		fp.qf.Start(feedCtx, fromHeight, ch)
		close(ch)
	}()

	for {
		select {
		case <-fp.daemonCtx.Done():
			log.Debug().Msg("closing feed processor daemon")
			feedCls()
			<-ch
			return
		case _, ok := <-ch:
			if !ok {
				log.Error().Msg("query feed closed unexpectedly")
				return
			}
			log.Debug().Msg("received query block")
		}
	}
}

func (fp *FeedProcessor) runBlockQueries(ctx context.Context, bqs BlockQueries) error {
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
	if lastHeight >= bqs.Height {
		return fmt.Errorf("last processed height %d isn't smaller than new height %d", lastHeight, bqs.Height)
	}

	// TODO(jsign)
	// Execute each query event and track the execution trace.
	//traces := make([]txn.TxnExecutionTrace, len(bqs.Queries))
	for i, q := range bqs.Queries {
		/* TODO(jsign)
		traces[i] = txn.TxnExecutionTrace{
			BlockNumber: bqs.BlockNumber,
			Query:       q,
			Error:       fp.executeQuery(ctx, b, q),
		}
		*/
		if err := fp.executeQuery(ctx, b, q); err != nil {
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
	if err := b.SetLastProcessedHeight(ctx, bqs.Height); err != nil {
		return fmt.Errorf("set new processed height %d: %s", bqs.Height, err)
	}

	if err := b.Commit(ctx); err != nil {
		return fmt.Errorf("committing changes: %s", err)
	}

	// TODO(jsign): metrics in general.
	//t.incrementRunSQLCount(ctx, ctrl)

	return nil
}

func (fp *FeedProcessor) executeQuery(ctx context.Context, b txn.Batch, query string) error {
	readStmt, mutatingStmts, err := fp.parser.ValidateRunSQL(query)
	if err != nil {
		return fmt.Errorf("validating query: %s", err)
	}
	if readStmt != nil {
		return errors.New("query is a read statement")
	}
	if err := b.ExecWriteQueries(ctx, mutatingStmts); err != nil {
		return fmt.Errorf("executing mutating-query: %s", err)
	}
	return nil
}
