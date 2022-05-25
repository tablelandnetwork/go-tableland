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
	"github.com/textileio/go-tableland/pkg/logging"
	"github.com/textileio/go-tableland/pkg/metrics"
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

		cp, err := counterprobe.New(
			chainCfg.Name,
			cfg.Target,
			chainCfg.Probe.SIWE,
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
