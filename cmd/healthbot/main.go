package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/buildinfo"
	"github.com/textileio/go-tableland/cmd/healthbot/counterprobe"
	"github.com/textileio/go-tableland/pkg/client"
	"github.com/textileio/go-tableland/pkg/logging"
	"github.com/textileio/go-tableland/pkg/metrics"
	"github.com/textileio/go-tableland/pkg/wallet"
)

func main() {
	cfg := setupConfig()
	logging.SetupLogger(buildinfo.GitCommit, cfg.Log.Debug, cfg.Log.Human)
	if err := metrics.SetupInstrumentation(":" + cfg.Metrics.Port); err != nil {
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
		wallet, err := wallet.NewWallet(chainCfg.Signer.PrivateKey)
		if err != nil {
			log.Fatal().Err(err).Msg("unable to create wallet from private key string")
		}

		chain, err := getChain(chainCfg.Name)
		if err != nil {
			log.Fatal().Err(err).Msg("unable to get chain")
		}

		client, err := client.NewClient(ctx, wallet, client.NewClientChain(chain))
		if err != nil {
			log.Fatal().Err(err).Msg("error creating tbl client")
		}

		cp, err := counterprobe.New(
			chainCfg.Name,
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

func getChain(chain string) (client.Chain, error) {
	switch chain {
	case "ethereum":
		return client.Chains.Ethereum, nil
	case "optimism":
		return client.Chains.Optimism, nil
	case "polygon":
		return client.Chains.Polygon, nil
	case "ethereum-goerli":
		return client.Chains.EthereumGoerli, nil
	case "optimism-kovan":
		return client.Chains.OptimismKovan, nil
	case "optimism-goerli":
		return client.Chains.OptimismGoerli, nil
	case "arbitrum-goerli":
		return client.Chains.ArbitrumGoerli, nil
	case "polygon-mumbai":
		return client.Chains.PolygonMumbai, nil
	case "local":
		return client.Chains.Local, nil
	default:
		return client.Chain{}, fmt.Errorf("%s is not a valid chain", chain)
	}
}
