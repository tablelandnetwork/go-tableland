package counterprobe

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"

	"github.com/ethereum/go-ethereum/rpc"
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

	rpcClient *rpc.Client

	lock                 sync.RWMutex
	mLastCounterValue    int64
	mLastCheck           time.Time
	mLastSuccessfulCheck time.Time
	mLatencyHist         syncint64.Histogram
	mBaseLabels          []attribute.KeyValue
}

// New returns a *CounterProbe.
func New(
	chainName string,
	endpoint string,
	siwe string,
	tableName string,
	checkInterval time.Duration,
	receiptTimeout time.Duration) (*CounterProbe, error) {
	log := logger.With().
		Str("component", "healthbot").
		Str("chainName", chainName).
		Logger()

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

	cp := &CounterProbe{
		log:            log,
		checkInterval:  checkInterval,
		rpcClient:      rpcClient,
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

	time.Sleep(time.Second * 30) // ~wait for the validator to spin-up
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
	updateCounterReq := tableland.RelayWriteQueryRequest{
		Statement: fmt.Sprintf("update %s set counter=counter+1", cp.tableName),
	}
	var updateCounterRes tableland.RelayWriteQueryResponse
	if err := cp.rpcClient.CallContext(ctx, &updateCounterRes, "tableland_relayWriteQuery", updateCounterReq); err != nil {
		return fmt.Errorf("calling tableland_runReadQuery: %s", err)
	}

	getReceiptRequest := tableland.GetReceiptRequest{
		TxnHash: updateCounterRes.Transaction.Hash,
	}

	start := time.Now()
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
			cp.log.Info().Int64("duration", time.Since(start).Milliseconds()).Msg("receipt confirmed")
			return nil
		}
		time.Sleep(time.Second * 5)
	}

	return fmt.Errorf("timed out waiting for receipt %s", getReceiptRequest.TxnHash)
}

func (cp *CounterProbe) getCurrentCounterValue(ctx context.Context) (int64, error) {
	getCounterReq := tableland.RunReadQueryRequest{
		Statement: fmt.Sprintf("select * from %s", cp.tableName),
	}

	type data struct {
		Rows [][]int64 `json:"rows"`
	}
	var getCounterRes struct {
		Result data `json:"data"`
	}
	if err := cp.rpcClient.CallContext(ctx, &getCounterRes, "tableland_runReadQuery", getCounterReq); err != nil {
		return 0, fmt.Errorf("calling tableland_runSQL: %s", err)
	}
	if len(getCounterRes.Result.Rows) != 1 || len(getCounterRes.Result.Rows[0]) != 1 {
		return 0, fmt.Errorf("unexpected response format")
	}

	return getCounterRes.Result.Rows[0][0], nil
}
