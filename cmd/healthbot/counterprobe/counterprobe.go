package counterprobe

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/client"
	"github.com/textileio/go-tableland/pkg/wallet"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

const (
	metricPrefix = "tableland.healthbot.e2eprobe"
)

// CounterProbe allows running an e2e probe for a pre-minted table
// that has a counter column.
type CounterProbe struct {
	log zerolog.Logger

	checkInterval  time.Duration
	receiptTimeout time.Duration
	tableName      string

	client *client.Client

	lock                 sync.RWMutex
	mLastCounterValue    int64
	mLastCheck           time.Time
	mLastSuccessfulCheck time.Time
	mLatencyHist         syncint64.Histogram
	mBaseLabels          []attribute.KeyValue
}

// New returns a *CounterProbe.
func New(
	ctx context.Context,
	chainName string,
	wallet *wallet.Wallet,
	tableName string,
	checkInterval time.Duration,
	receiptTimeout time.Duration,
) (*CounterProbe, error) {
	log := logger.With().
		Str("component", "healthbot").
		Str("chain_name", chainName).
		Logger()

	if wallet == nil {
		return nil, errors.New("wallet can't be nil")
	}
	if receiptTimeout == 0 {
		return nil, errors.New("receipt timeout can't be zero")
	}
	if len(tableName) == 0 {
		return nil, errors.New("tablename is empty")
	}

	client, err := client.NewClient(ctx, wallet)
	if err != nil {
		return nil, fmt.Errorf("creating tbl client: %v", err)
	}

	cp := &CounterProbe{
		log:            log,
		checkInterval:  checkInterval,
		client:         client,
		tableName:      tableName,
		receiptTimeout: receiptTimeout,
	}
	if err := cp.initMetrics(chainName); err != nil {
		return nil, fmt.Errorf("initializing metrics: %s", err)
	}

	return cp, nil
}

// Run runs the probe until the provided ctx is canceled.
func (cp *CounterProbe) Run(ctx context.Context) {
	cp.log.Info().Msg("starting counter-probe...")

	time.Sleep(time.Second * 15) // ~wait for the validator to spin-up
	if err := cp.execProbe(ctx); err != nil {
		cp.log.Error().Err(err).Msg("health check failed")
	}
	for {
		select {
		case <-ctx.Done():
			cp.log.Info().Msg("closing gracefully...")
			return
		case <-time.After(cp.checkInterval):
			if err := cp.execProbe(ctx); err != nil {
				cp.log.Error().Err(err).Msg("health check failed")
			}
		}
	}
}

func (cp *CounterProbe) execProbe(ctx context.Context) error {
	cp.lock.Lock()
	cp.mLastCheck = time.Now()
	cp.lock.Unlock()

	counterValue, err := cp.healthCheck(ctx)
	if err != nil {
		return fmt.Errorf("health check: %s", err)
	}

	cp.mLatencyHist.Record(ctx, time.Since(cp.mLastCheck).Milliseconds(), cp.mBaseLabels...)
	cp.lock.Lock()
	cp.mLastSuccessfulCheck = time.Now()
	cp.mLastCounterValue = counterValue
	cp.lock.Unlock()

	return nil
}

func (cp *CounterProbe) healthCheck(ctx context.Context) (int64, error) {
	currentCounter, err := cp.getCurrentCounterValue(ctx)
	if err != nil {
		return 0, fmt.Errorf("get current counter value: %s", err)
	}
	if err := cp.increaseCounterValue(ctx); err != nil {
		return 0, fmt.Errorf("increasing counter value: %s", err)
	}
	updatedCounter, err := cp.getCurrentCounterValue(ctx)
	if err != nil {
		return 0, fmt.Errorf("updated counter value: %s", err)
	}

	if updatedCounter != currentCounter+1 {
		return 0, fmt.Errorf("unexpected updated counter value (exp: %d, got: %d)", currentCounter+1, updatedCounter)
	}

	return updatedCounter, nil
}

func (cp *CounterProbe) increaseCounterValue(ctx context.Context) error {
	hash, err := cp.client.Write(
		ctx,
		fmt.Sprintf("update %s set counter=counter+1", cp.tableName),
		client.WriteRelay(true),
	)
	if err != nil {
		return fmt.Errorf("calling client write: %s", err)
	}
	start := time.Now()
	receipt, ok, err := cp.client.Receipt(ctx, hash, client.WaitFor(cp.receiptTimeout))
	if err != nil {
		return fmt.Errorf("getting receipt: %s", err)
	}
	if !ok {
		return errors.New("receipt not found before timeout")
	}
	if receipt.Error != "" {
		return fmt.Errorf("receipt found but has an error %s", receipt.Error)
	}
	cp.log.Info().Int64("duration", time.Since(start).Milliseconds()).Msg("receipt confirmed")
	return nil
}

func (cp *CounterProbe) getCurrentCounterValue(ctx context.Context) (int64, error) {
	var counter int64
	if err := cp.client.Read(
		ctx,
		fmt.Sprintf("select counter from %s", cp.tableName),
		&counter,
		client.ReadExtract(),
		client.ReadUnwrap(),
	); err != nil {
		return 0, fmt.Errorf("calling client read: %s", err)
	}
	return counter, nil
}
