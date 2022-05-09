package counterprobe

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"

	"github.com/ethereum/go-ethereum/rpc"
)

const (
	metricPrefix = "tableland.healthbot.e2eprobe"
)

// CounterProbe allows running an e2e probe for a pre-minted table
// that has a counter column.
type CounterProbe struct {
	checkInterval  time.Duration
	receiptTimeout time.Duration
	tableName      string

	rpcClient *rpc.Client

	lock                 sync.RWMutex
	mLastCounterValue    int64
	mLastCheck           time.Time
	mLastSuccessfulCheck time.Time
	mLatencyHist         syncint64.Histogram
}

// New returns a *CounterProbe.
func New(endpoint string,
	siwe string,
	tableName string,
	checkInterval time.Duration,
	receiptTimeout time.Duration) (*CounterProbe, error) {
	if receiptTimeout == 0 {
		return nil, fmt.Errorf("receipt timeout can't be zero")
	}
	if len(tableName) == 0 {
		return nil, errors.New("tablename is empty")
	}
	if _, err := url.ParseQuery(endpoint); err != nil {
		return nil, fmt.Errorf("invalid endpoint target: %s", err)
	}
	rpcClient, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, fmt.Errorf("creating jsonrpc client: %s", err)
	}
	rpcClient.SetHeader("Authorization", "Bearer "+siwe)

	meter := global.MeterProvider().Meter("tableland")
	latencyHistogram, err := meter.SyncInt64().Histogram(metricPrefix + ".latency")
	if err != nil {
		return &CounterProbe{}, fmt.Errorf("registering latency histogram: %s", err)
	}

	cp := &CounterProbe{
		checkInterval:  checkInterval,
		rpcClient:      rpcClient,
		tableName:      tableName,
		receiptTimeout: receiptTimeout,

		mLatencyHist: latencyHistogram,
	}

	mLastCheck, err := meter.AsyncInt64().Gauge(metricPrefix + ".last_check")
	if err != nil {
		return &CounterProbe{}, fmt.Errorf("registering last check gauge: %s", err)
	}

	mLastSuccessfulCheck, err := meter.AsyncInt64().Gauge(metricPrefix + ".last_successful_check")
	if err != nil {
		return &CounterProbe{}, fmt.Errorf("registering last full check gauge: %s", err)
	}

	mCounterValue, err := meter.AsyncInt64().Gauge(metricPrefix + ".counter_value")
	if err != nil {
		return &CounterProbe{}, fmt.Errorf("registering counter value gauge: %s", err)
	}

	instruments := []instrument.Asynchronous{mLastCheck, mLastSuccessfulCheck, mCounterValue}
	if err := meter.RegisterCallback(instruments, func(ctx context.Context) {
		cp.lock.RLock()
		defer cp.lock.RUnlock()

		mLastCheck.Observe(ctx, cp.mLastCheck.Unix())
		mLastSuccessfulCheck.Observe(ctx, cp.mLastSuccessfulCheck.Unix())
		mCounterValue.Observe(ctx, cp.mLastCounterValue)
	}); err != nil {
		return &CounterProbe{}, fmt.Errorf("registering callback on instruments: %s", err)
	}

	return cp, nil
}

// Run runs the probe until the provided ctx is canceled.
func (cp *CounterProbe) Run(ctx context.Context) {
	log.Info().Msg("starting counter-probe...")
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("closing gracefully...")
			return
		case <-time.After(cp.checkInterval):
			if err := cp.execProbe(ctx); err != nil {
				log.Error().Err(err).Msg("health check failed")
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

	cp.mLatencyHist.Record(ctx, time.Since(cp.mLastCheck).Milliseconds())
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
	updateCounterReq := tableland.RunSQLRequest{
		Statement: fmt.Sprintf("update %s set counter=counter+1", cp.tableName),
	}
	var updateCounterRes tableland.RunSQLResponse
	if err := cp.rpcClient.CallContext(ctx, &updateCounterRes, "tableland_runSQL", updateCounterReq); err != nil {
		return fmt.Errorf("calling tableland_runSQL: %s", err)
	}

	getReceiptRequest := tableland.GetReceiptRequest{
		TxnHash: updateCounterRes.Transaction.Hash,
	}

	deadline := time.Now().Add(cp.receiptTimeout)
	for time.Now().Before(deadline) {
		var getReceiptResponse tableland.GetReceiptResponse
		if err := cp.rpcClient.CallContext(ctx, &getReceiptResponse, "tableland_getReceipt", getReceiptRequest); err != nil {
			return fmt.Errorf("calling tableland_getReceipt: %s", err)
		}
		if getReceiptResponse.Ok {
			if getReceiptResponse.Receipt.Error != nil {
				return fmt.Errorf("receipt found but has an error %s", *getReceiptResponse.Receipt.Error)
			}
			return nil
		}
		time.Sleep(time.Second * 5)
	}

	return fmt.Errorf("timed out waiting for receipt %s", getReceiptRequest.TxnHash)
}

func (cp *CounterProbe) getCurrentCounterValue(ctx context.Context) (int64, error) {
	getCounterReq := tableland.RunSQLRequest{
		Statement: fmt.Sprintf("select * from %s", cp.tableName),
	}

	type data struct {
		Rows [][]int64 `json:"rows"`
	}
	var getCounterRes struct {
		Result data `json:"data"`
	}
	if err := cp.rpcClient.CallContext(ctx, &getCounterRes, "tableland_runSQL", getCounterReq); err != nil {
		return 0, fmt.Errorf("calling tableland_runSQL: %s", err)
	}
	if len(getCounterRes.Result.Rows) != 1 || len(getCounterRes.Result.Rows[0]) != 1 {
		return 0, fmt.Errorf("unexpected response format")
	}

	return getCounterRes.Result.Rows[0][0], nil
}
