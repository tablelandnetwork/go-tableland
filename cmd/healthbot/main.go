package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/buildinfo"
	"github.com/textileio/go-tableland/cmd/healthbot/counterprobe"
	"github.com/textileio/go-tableland/pkg/client"
	clientV1 "github.com/textileio/go-tableland/pkg/client/v1"
	"github.com/textileio/go-tableland/pkg/logging"
	"github.com/textileio/go-tableland/pkg/metrics"
	"github.com/textileio/go-tableland/pkg/wallet"
)

func main() {
	cfg := setupConfig()
	logging.SetupLogger(buildinfo.GitCommit, cfg.Log.Debug, cfg.Log.Human)
	if err := metrics.SetupInstrumentation(":"+cfg.Metrics.Port, "tableland:healthbot"); err != nil {
		log.Fatal().Err(err).Str("port", cfg.Metrics.Port).Msg("could not setup instrumentation")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	var wg sync.WaitGroup
	for _, chainCfg := range cfg.Chains {
		checkInterval, err := time.ParseDuration(chainCfg.Probe.CheckInterval)
		if err != nil {
			log.Fatal().Err(err).Msgf("check interval has invalid format: %s", chainCfg.Probe.CheckInterval)
		}
		receiptTimeout, err := time.ParseDuration(chainCfg.Probe.ReceiptTimeout)
		if err != nil {
			log.Fatal().Err(err).Msgf("receipt timeout has invalid format: %s", chainCfg.Probe.ReceiptTimeout)
		}

		wallet, err := wallet.NewWallet(chainCfg.WalletPrivateKey)
		if err != nil {
			log.Fatal().Err(err).Msg("unable to create wallet from private key string")
		}

		chain, ok := client.Chains[client.ChainID(chainCfg.ChainID)]
		if !ok {
			log.Fatal().Int("chain_id", chainCfg.ChainID).Msg("the chain id isn't supported in the Tableland client")
		}

		if chainCfg.OverrideClient.GatewayEndpoint != "" {
			chain.Endpoint = chainCfg.OverrideClient.GatewayEndpoint
		}

		if chainCfg.OverrideClient.ContractAddr != "" {
			chain.ContractAddr = common.HexToAddress(chainCfg.OverrideClient.ContractAddr)
		}

		// For Filecoin Hyperspace, we use Ankr endpoint
		opts := []clientV1.NewClientOption{clientV1.NewClientChain(chain)}
		if chain.ID == 3141 {
			opts = append(opts, clientV1.NewClientAnkrAPIKey(chainCfg.AnkrAPIKey))
		} else {
			opts = append(opts, clientV1.NewClientAlchemyAPIKey(chainCfg.AlchemyAPIKey))
		}
		client, err := clientV1.NewClient(ctx, wallet, opts...)
		if err != nil {
			log.Fatal().Err(err).Msg("error creating tbl client")
		}

		suggestedGasPriceMultiplier := 1.0
		if chainCfg.OverrideClient.SuggestedGasPriceMultiplier > 0 {
			suggestedGasPriceMultiplier = chainCfg.OverrideClient.SuggestedGasPriceMultiplier
		}
		estimatedGasLimitMultiplier := 1.0
		if chainCfg.OverrideClient.EstimatedGasLimitMultiplier > 0 {
			estimatedGasLimitMultiplier = chainCfg.OverrideClient.EstimatedGasLimitMultiplier
		}

		cp, err := counterprobe.New(
			chain.Name,
			client,
			chainCfg.Probe.Tablename,
			checkInterval,
			receiptTimeout,
			suggestedGasPriceMultiplier,
			estimatedGasLimitMultiplier,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("initializing counter-probe")
		}

		balanceTracker, err := NewBalanceTracker(
			chainCfg,
			wallet,
			15*time.Second,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("initializing balance tracker")
		}

		wg.Add(2)
		go func() {
			defer wg.Done()
			cp.Run(ctx)
		}()

		go func() {
			defer wg.Done()
			balanceTracker.Run(ctx)
		}()
	}
	wg.Wait()
	log.Info().Msg("daemon closed")
}
