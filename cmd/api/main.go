package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	"github.com/textileio/cli"
	"github.com/textileio/go-tableland/buildinfo"
	"github.com/textileio/go-tableland/internal/chains"
	"github.com/textileio/go-tableland/internal/router"
	systemimpl "github.com/textileio/go-tableland/internal/system/impl"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/internal/tableland/impl"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	epimpl "github.com/textileio/go-tableland/pkg/eventprocessor/impl"
	"github.com/textileio/go-tableland/pkg/logging"
	"github.com/textileio/go-tableland/pkg/metrics"
	nonceimpl "github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/user"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/txn"
	txnimpl "github.com/textileio/go-tableland/pkg/txn/impl"
	"github.com/textileio/go-tableland/pkg/wallet"
)

func main() {
	config := setupConfig()
	logging.SetupLogger(buildinfo.GitCommit, config.Log.Debug, config.Log.Human)

	// Validator instrumentation configuration.
	if err := metrics.SetupInstrumentation(":" + config.Metrics.Port); err != nil {
		log.Fatal().
			Err(err).
			Str("port", config.Metrics.Port).
			Msg("could not setup instrumentation")
	}

	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&timezone=UTC",
		config.DB.User,
		config.DB.Pass,
		config.DB.Host,
		config.DB.Port,
		config.DB.Name,
	)

	parserOpts := []parsing.Option{
		parsing.WithMaxAllowedColumns(config.TableConstraints.MaxColumns),
		parsing.WithMaxTextLength(config.TableConstraints.MaxColumns),
		parsing.WithMaxReadQuerySize(config.QueryConstraints.MaxReadQuerySize),
		parsing.WithMaxWriteQuerySize(config.QueryConstraints.MaxWriteQuerySize),
	}

	parser, err := parserimpl.New([]string{systemimpl.SystemTablesPrefix, systemimpl.RegistryTableName}, parserOpts...)
	if err != nil {
		log.Fatal().Err(err).Msg("new parser")
	}

	parser, err = parserimpl.NewInstrumentedSQLValidator(parser)
	if err != nil {
		log.Fatal().Err(err).Msg("instrumenting sql validator")
	}

	chainStacks := map[tableland.ChainID]chains.ChainStack{}
	for _, chainCfg := range config.Chains {
		if _, ok := chainStacks[chainCfg.ChainID]; ok {
			log.Fatal().Int64("chainId", int64(chainCfg.ChainID)).Msg("chain id configuration is duplicated")
		}
		chainStack, err := createChainIDStack(chainCfg, databaseURL, parser, config.TableConstraints)
		if err != nil {
			log.Fatal().Int64("chainId", int64(chainCfg.ChainID)).Err(err).Msg("spinning up chain stack")
		}
		chainStacks[chainCfg.ChainID] = chainStack
	}

	userStore, err := user.New(databaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("creating user store")
	}

	rateLimInterval, err := time.ParseDuration(config.HTTP.RateLimInterval)
	if err != nil {
		log.Fatal().Err(err).Msg("parsing http rate lim interval")
	}

	router := router.ConfiguredRouter(
		config.Gateway.ExternalURIPrefix,
		config.HTTP.MaxRequestPerInterval,
		rateLimInterval,
		parser,
		userStore,
		chainStacks,
	)

	server := &http.Server{
		Addr:         ":" + config.HTTP.Port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      router.Handler(),
	}

	// Admin endpoint configuration.
	if config.AdminAPI.Password == "" {
		log.Warn().
			Msg("no admin api password set")
	}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				log.Info().Msg("http serve gracefully closed")
				return
			}
			log.Fatal().
				Err(err).
				Str("port", config.HTTP.Port).
				Msg("could not start server")
		}
	}()

	cli.HandleInterrupt(func() {
		var wg sync.WaitGroup
		wg.Add(len(chainStacks))
		for chainID, stack := range chainStacks {
			go func(chainID tableland.ChainID, stack chains.ChainStack) {
				defer wg.Done()

				ctx, cls := context.WithTimeout(context.Background(), time.Second*15)
				defer cls()
				if err := stack.Close(ctx); err != nil {
					log.Error().Err(err).Int64("chainID", int64(chainID)).Msg("finalizing chain stack")
				}
			}(chainID, stack)
		}
		wg.Wait()

		ctx, cls := context.WithTimeout(context.Background(), time.Second*10)
		defer cls()
		if err := server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("shutting down http server")
		}

		if err := userStore.Close(); err != nil {
			log.Error().Err(err).Msg("closing user store")
		}
	})
}

func createChainIDStack(
	config ChainConfig,
	databaseURL string,
	parser parsing.SQLValidator,
	tableConstraints TableConstraints,
) (chains.ChainStack, error) {
	store, err := system.New(databaseURL, config.ChainID)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("failed initialize sqlstore: %s", err)
	}

	systemStore, err := sqlstoreimpl.NewInstrumentedSystemStore(config.ChainID, store)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("instrumenting system store pgx: %s", err)
	}

	conn, err := ethclient.Dial(config.Registry.EthEndpoint)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("failed to connect to ethereum endpoint: %s", err)
	}

	wallet, err := wallet.NewWallet(config.Signer.PrivateKey)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("failed to create wallet: %s", err)
	}
	log.Info().
		Int64("chainID", int64(config.ChainID)).
		Str("wallet", wallet.Address().String()).
		Msg("wallet public address")

	checkInterval, err := time.ParseDuration(config.NonceTracker.CheckInterval)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("parsing nonce tracker check interval duration: %s", err)
	}
	stuckInterval, err := time.ParseDuration(config.NonceTracker.StuckInterval)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("parsing nonce tracker stuck interval duration: %s", err)
	}
	ctxLocalTracker, clsLocalTracker := context.WithTimeout(context.Background(), time.Second*15)
	defer clsLocalTracker()
	tracker, err := nonceimpl.NewLocalTracker(
		ctxLocalTracker,
		wallet,
		nonceimpl.NewNonceStore(systemStore),
		config.ChainID,
		conn,
		checkInterval,
		config.NonceTracker.MinBlockDepth,
		stuckInterval,
	)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("failed to create new tracker: %s", err)
	}

	scAddress := common.HexToAddress(config.Registry.ContractAddress)
	registry, err := ethereum.NewClient(
		conn,
		config.ChainID,
		scAddress,
		wallet,
		tracker,
	)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("failed to create ethereum client: %s", err)
	}

	acl := impl.NewACL(systemStore, registry)

	var txnp txn.TxnProcessor
	txnp, err = txnimpl.NewTxnProcessor(config.ChainID, databaseURL, tableConstraints.MaxRowCount, acl)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("creating txn processor: %s", err)
	}
	chainAPIBackoff, err := time.ParseDuration(config.EventFeed.ChainAPIBackoff)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("parsing chain api backoff duration: %s", err)
	}
	newBlockTimeout, err := time.ParseDuration(config.EventFeed.NewBlockTimeout)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("parsing chain api backoff duration: %s", err)
	}
	efOpts := []eventfeed.Option{
		eventfeed.WithChainAPIBackoff(chainAPIBackoff),
		eventfeed.WithMaxBlocksFetchSize(config.EventFeed.MaxBlocksFetchSize),
		eventfeed.WithMinBlockDepth(config.EventFeed.MinBlockDepth),
		eventfeed.WithNewBlockTimeout(newBlockTimeout),
	}
	ef, err := efimpl.New(config.ChainID, conn, scAddress, efOpts...)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("creating event feed: %s", err)
	}
	blockFailedExecutionBackoff, err := time.ParseDuration(config.EventProcessor.BlockFailedExecutionBackoff)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("parsing block failed execution backoff duration: %s", err)
	}
	epOpts := []eventprocessor.Option{
		eventprocessor.WithBlockFailedExecutionBackoff(blockFailedExecutionBackoff),
		eventprocessor.WithDedupExecutedTxns(config.EventProcessor.DedupExecutedTxns),
	}
	ep, err := epimpl.New(parser, txnp, ef, config.ChainID, epOpts...)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("creating event processor")
	}
	if err := ep.Start(); err != nil {
		return chains.ChainStack{}, fmt.Errorf("starting event processor")
	}

	return chains.ChainStack{
		Store:    systemStore,
		Registry: registry,
		Close: func(ctx context.Context) error {
			log.Info().Int64("chainId", int64(config.ChainID)).Msg("closing stack...")
			defer log.Info().Int64("chainId", int64(config.ChainID)).Msg("stack closed")

			ep.Stop()
			conn.Close()
			if err := systemStore.Close(); err != nil {
				log.Error().Int64("chainId", int64(config.ChainID)).Err(err).Msg("closing system store")
			}
			return nil
		},
	}, nil
}
