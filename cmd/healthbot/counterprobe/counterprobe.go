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
	jwtp "github.com/textileio/go-tableland/pkg/jwt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"

	"github.com/ethereum/go-ethereum/rpc"
)

const (
	metricPrefix = "tableland.healthbot.e2eprobe"
)

type CounterProbe struct {
	chckInterval time.Duration
	rpcClient    *rpc.Client
	ctrl         string
	tblname      string

	lock                sync.RWMutex
	lastCounterValue    int64
	lastCheck           time.Time
	lastSuccessfulCheck time.Time

	latencyHist metric.Int64Histogram
}

func New(chckInterval time.Duration, endpoint, jwt, tblname string) (*CounterProbe, error) {
	if len(tblname) == 0 {
		return nil, errors.New("tablename is empty")
	}
	if _, err := url.ParseQuery(endpoint); err != nil {
		return nil, fmt.Errorf("invalid endpoint target: %s", err)
	}
	j, err := jwtp.Parse(jwt)
	if err != nil {
		return nil, fmt.Errorf("invalid jwt: %s", err)
	}
	if err := j.Verify(); err != nil {
		return nil, fmt.Errorf("validating jwt: %s", err)
	}
	if j.Claims.Issuer == "" {
		return nil, errors.New("jwt has no issuer")
	}
	rpcClient, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, fmt.Errorf("creating jsonrpc client: %s", err)
	}
	rpcClient.SetHeader("Authorization", "Bearer "+jwt)

	meter := metric.Must(global.Meter("tableland"))
	cp := &CounterProbe{
		chckInterval: chckInterval,
		rpcClient:    rpcClient,
		ctrl:         j.Claims.Issuer,
		tblname:      tblname,

		latencyHist: meter.NewInt64Histogram(metricPrefix + ".latency"),
	}
	var mLastCheck metric.Int64GaugeObserver
	var mLastSuccessfulCheck metric.Int64GaugeObserver
	var mCounterValue metric.Int64GaugeObserver
	batchObs := meter.NewBatchObserver(func(ctx context.Context, r metric.BatchObserverResult) {
		cp.lock.RLock()
		defer cp.lock.RUnlock()

		obs := []metric.Observation{
			mLastCheck.Observation(cp.lastCheck.Unix()),
			mLastSuccessfulCheck.Observation(cp.lastSuccessfulCheck.Unix()),
			mCounterValue.Observation(cp.lastCounterValue),
		}
		r.Observe([]attribute.KeyValue{}, obs...)

	})
	mLastCheck = batchObs.NewInt64GaugeObserver(metricPrefix + ".last_check")
	mLastSuccessfulCheck = batchObs.NewInt64GaugeObserver(metricPrefix + ".last_successful_check")
	mCounterValue = batchObs.NewInt64GaugeObserver(metricPrefix + ".counter_value")

	return cp, nil
}

func (cp *CounterProbe) Run(ctx context.Context) {
	log.Info().Msg("starting counter-probe...")
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("closing gracefully...")
			return
		case <-time.After(cp.chckInterval):
			if err := cp.execProbe(ctx); err != nil {
				log.Error().Err(err).Msg("health check failed")
			}
		}
	}
}

func (cp *CounterProbe) execProbe(ctx context.Context) error {
	cp.lock.Lock()
	defer cp.lock.Unlock()

	cp.lastCheck = time.Now()
	counterValue, err := cp.healthCheck(ctx)
	if err != nil {
		return fmt.Errorf("health check: %s", err)
	}
	cp.lastSuccessfulCheck = time.Now()
	cp.latencyHist.Record(ctx, time.Since(cp.lastCheck).Milliseconds())
	cp.lastCounterValue = counterValue
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
		Controller: cp.ctrl,
		Statement:  fmt.Sprintf("update %s set count=count+1", cp.tblname),
	}
	var updateCounterRes tableland.RunSQLResponse
	if err := cp.rpcClient.CallContext(ctx, &updateCounterRes, "tableland_runSQL", updateCounterReq); err != nil {
		return fmt.Errorf("calling tableland_runSQL: %s", err)
	}

	return nil
}

func (cp *CounterProbe) getCurrentCounterValue(ctx context.Context) (int64, error) {
	getCounterReq := tableland.RunSQLRequest{
		Controller: cp.ctrl,
		Statement:  fmt.Sprintf("select * from %s", cp.tblname),
	}

	type Data struct {
		Rows [][]int64 `json:"rows"`
	}
	var getCounterRes struct {
		Result Data `json:"data"`
	}
	if err := cp.rpcClient.CallContext(ctx, &getCounterRes, "tableland_runSQL", getCounterReq); err != nil {
		return 0, fmt.Errorf("calling tableland_runSQL: %s", err)
	}
	if len(getCounterRes.Result.Rows) != 1 || len(getCounterRes.Result.Rows[0]) != 1 {
		return 0, fmt.Errorf("unexpected response format")
	}

	return getCounterRes.Result.Rows[0][0], nil
}
