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
)

func main() {
	cfg := setupConfig()
	logging.SetupLogger(buildinfo.GitCommit, cfg.Log.Debug, cfg.Log.Human)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	// Check interval format.
	checkInterval, err := time.ParseDuration(cfg.Probe.CheckInterval)
	if err != nil {
		log.Fatal().Err(err).Msgf("check interval has invalid format: %s", cfg.Probe.CheckInterval)
	}

	cp, err := counterprobe.New(checkInterval, cfg.Probe.Endpoint, cfg.Probe.JWT, cfg.Probe.Tablename)
	if err != nil {
		log.Fatal().Err(err).Msg("initializing counter-probe")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		cp.Run(ctx)
	}()

	wg.Wait()
	log.Info().Msg("closing daemon")
}
