package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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

		client, err := clientV1.NewClient(
			ctx, wallet, clientV1.NewClientChain(chain), clientV1.NewClientAlchemyAPIKey(chainCfg.AlchemyAPIKey))
		if err != nil {
			log.Fatal().Err(err).Msg("error creating tbl client")
		}

		cp, err := counterprobe.New(
			chain.Name,
			client,
			chainCfg.Probe.Tablename,
			checkInterval,
			receiptTimeout)
		if err != nil {
			log.Fatal().Err(err).Msg("initializing counter-probe")
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			cp.Run(ctx)
		}()
	}
	wg.Wait()
	log.Info().Msg("daemon closed")
}
