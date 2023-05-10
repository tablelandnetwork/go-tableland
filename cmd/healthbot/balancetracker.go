package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/client"
	"github.com/textileio/go-tableland/pkg/metrics"
	"github.com/textileio/go-tableland/pkg/wallet"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

// BalanceTracker tracks the balance of a given wallet and produces metrics.
type BalanceTracker struct {
	checkInterval time.Duration
	wallet        *wallet.Wallet
	ethClient     *ethclient.Client

	log zerolog.Logger

	mu sync.Mutex

	// metrics
	mBaseLabels        []attribute.KeyValue
	currWeiBalance     int64
	ethClientUnhealthy int64
}

// NewBalanceTracker returns a *BalanceTracker.
func NewBalanceTracker(
	config ChainConfig,
	wallet *wallet.Wallet,
	checkInterval time.Duration,
) (*BalanceTracker, error) {
	log := logger.With().
		Str("component", "healthbot").
		Int("chain_id", config.ChainID).
		Logger()

	client, err := getEthClient(config)
	if err != nil {
		return nil, fmt.Errorf("initializing eth client: %s", err)
	}

	cp := &BalanceTracker{
		log:           log,
		checkInterval: checkInterval,
		ethClient:     client,
		wallet:        wallet,
	}
	if err := cp.initMetrics(config.ChainID, wallet.Address()); err != nil {
		return nil, fmt.Errorf("initializing metrics: %s", err)
	}

	return cp, nil
}

// Run runs the probe until the provided ctx is canceled.
func (t *BalanceTracker) Run(ctx context.Context) {
	t.log.Info().Msg("starting balance tracker...")

	if err := t.checkBalance(ctx); err != nil {
		t.log.Error().Err(err).Msg("check balance failed")
	}

	checkInterval := t.checkInterval
	for {
		select {
		case <-ctx.Done():
			t.log.Info().Msg("closing gracefully...")
			return
		case <-time.After(checkInterval):
			if err := t.checkBalance(ctx); err != nil {
				t.log.Error().Err(err).Msg("check balance failed")
				checkInterval = time.Minute
			} else {
				checkInterval = t.checkInterval
			}
		}
	}
}

func (t *BalanceTracker) checkBalance(ctx context.Context) error {
	ctx, cls := context.WithTimeout(ctx, time.Second*15)
	defer cls()
	weiBalance, err := t.ethClient.BalanceAt(ctx, t.wallet.Address(), nil)
	if err != nil {
		t.mu.Lock()
		t.ethClientUnhealthy++
		t.mu.Unlock()
		return fmt.Errorf("get balance: %s", err)
	}

	t.log.Info().
		Str("balance", weiBalance.String()).
		Str("address", t.wallet.Address().Hex()).
		Msg("check balance")

	s := weiBalance.String()
	var gWeiBalance int64
	if len(s) > 9 {
		gWeiBalance, err = strconv.ParseInt(s[:len(s)-9], 10, 64)
		if err != nil {
			return fmt.Errorf("converting wei to gwei: %s", err)
		}
	}

	t.mu.Lock()
	t.currWeiBalance = gWeiBalance
	t.ethClientUnhealthy = 0
	t.mu.Unlock()

	return nil
}

func (t *BalanceTracker) initMetrics(chainID int, addr common.Address) error {
	meter := global.MeterProvider().Meter("tableland")
	t.mBaseLabels = append([]attribute.KeyValue{
		attribute.Int("chain_id", chainID),
		attribute.String("wallet_address", addr.String()),
	}, metrics.BaseAttrs...)

	mBalance, err := meter.Int64ObservableGauge("tableland.wallettracker.balance.wei")
	if err != nil {
		return fmt.Errorf("creating balance metric: %s", err)
	}

	mEthClientUnhealthy, err := meter.Int64ObservableGauge("tableland.wallettracker.eth.client.unhealthy")
	if err != nil {
		return fmt.Errorf("creating eth client unhealthy metric: %s", err)
	}

	if _, err = meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			t.mu.Lock()
			defer t.mu.Unlock()
			o.ObserveInt64(mBalance, t.currWeiBalance, t.mBaseLabels...)
			o.ObserveInt64(mEthClientUnhealthy, t.ethClientUnhealthy, t.mBaseLabels...)

			return nil
		}, []instrument.Asynchronous{
			mBalance,
			mEthClientUnhealthy,
		}...); err != nil {
		return fmt.Errorf("registering async metric callback: %s", err)
	}

	return nil
}

func getEthClient(config ChainConfig) (*ethclient.Client, error) {
	var url, key string
	var ok bool
	if config.ChainID == 3141 {
		url, ok = client.AnkrURLs[client.ChainID(config.ChainID)]
		key = config.AnkrAPIKey
	} else {
		url, ok = client.AlchemyURLs[client.ChainID(config.ChainID)]
		key = config.AlchemyAPIKey
	}

	if !ok {
		return nil, errors.New("chain provider not supported")
	}

	conn, err := ethclient.Dial(fmt.Sprintf(url, key))
	if err != nil {
		return nil, fmt.Errorf("dial: %s", err)
	}

	return conn, nil
}
