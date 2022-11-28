package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"github.com/textileio/cli"
	"github.com/textileio/go-tableland/buildinfo"
	"github.com/textileio/go-tableland/internal/chains"
	"github.com/textileio/go-tableland/internal/router"
	systemimpl "github.com/textileio/go-tableland/internal/system/impl"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/internal/tableland/impl"
	"github.com/textileio/go-tableland/pkg/backup"
	"github.com/textileio/go-tableland/pkg/backup/restorer"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	epimpl "github.com/textileio/go-tableland/pkg/eventprocessor/impl"
	executor "github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor/impl"
	"github.com/textileio/go-tableland/pkg/logging"
	"github.com/textileio/go-tableland/pkg/metrics"
	nonceimpl "github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/user"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/telemetry"
	"github.com/textileio/go-tableland/pkg/telemetry/chainscollector"
	"github.com/textileio/go-tableland/pkg/telemetry/publisher"
	"github.com/textileio/go-tableland/pkg/telemetry/storage"
	"github.com/textileio/go-tableland/pkg/wallet"
	"go.opentelemetry.io/otel/attribute"
)

type moduleCloser func(ctx context.Context) error

var closerNoop = func(context.Context) error { return nil }

func main() {
	config, dirPath := setupConfig()

	// Logging.
	logging.SetupLogger(buildinfo.GitCommit, config.Log.Debug, config.Log.Human)

	// Instrumentation.
	if err := metrics.SetupInstrumentation(":"+config.Metrics.Port, "tableland:api"); err != nil {
		log.Fatal().Err(err).Str("port", config.Metrics.Port).Msg("could not setup instrumentation")
	}

	// Database URL.
	databaseURL := fmt.Sprintf(
		"file://%s?_busy_timeout=5000&_foreign_keys=on&_journal_mode=WAL",
		path.Join(dirPath, "database.db"),
	)

	// Restore provided backup (if configured).
	if config.BootstrapBackupURL != "" {
		if err := restoreBackup(databaseURL, config.BootstrapBackupURL); err != nil {
			log.Fatal().Err(err).Msg("restoring backup")
		}
	}

	// Parser.
	parser, err := createParser(config.QueryConstraints)
	if err != nil {
		log.Fatal().Err(err).Msg("creating parser")
	}

	// Chain stacks.
	chainStacks, closeChainStacks, err := createChainStacks(
		databaseURL,
		parser,
		config.Chains,
		config.TableConstraints,
		config.Analytics.FetchExtraBlockInfo)
	if err != nil {
		log.Fatal().Err(err).Msg("creating chains stack")
	}

	// User store.
	userStore, err := user.New(databaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("creating user store")
	}

	// HTTP API server.
	closeHTTPServer, err := createHTTPServer(config.HTTP, config.Gateway, parser, userStore, chainStacks)
	if err != nil {
		log.Fatal().Err(err).Msg("creating HTTP server")
	}

	// Backuper.
	closeBackupScheduler := closerNoop
	if config.Backup.Enabled {
		closeBackupScheduler, err = createBackuper(dirPath, config.Backup)
		if err != nil {
			log.Fatal().Err(err).Msg("creating backuper")
		}
	}

	// Telemetry
	closeTelemetryModule, err := configureTelemetry(dirPath, chainStacks, config.TelemetryPublisher)
	if err != nil {
		log.Fatal().Err(err).Msg("configuring telemetry")
	}

	cli.HandleInterrupt(func() {
		// Close HTTP server.
		ctx, cls := context.WithTimeout(context.Background(), time.Second*10)
		defer cls()
		if err := closeHTTPServer(ctx); err != nil {
			log.Error().Err(err).Msg("shutting down http server")
		}

		// Close chains syncing.
		ctx, cls = context.WithTimeout(context.Background(), time.Second*20)
		defer cls()
		if err := closeChainStacks(ctx); err != nil {
			log.Error().Err(err).Msg("closing chains stack")
		}

		// Close backuper.
		ctx, cls = context.WithTimeout(context.Background(), time.Second*20)
		defer cls()
		if err := closeBackupScheduler(ctx); err != nil {
			log.Error().Err(err).Msg("closing backuper")
		}

		// Close user store.
		if err := userStore.Close(); err != nil {
			log.Error().Err(err).Msg("closing user store")
		}

		// Close telemetry.
		if err := closeTelemetryModule(ctx); err != nil {
			log.Error().Err(err).Msg("closing telemetry module")
		}
	})
}

func createChainIDStack(
	config ChainConfig,
	dbURI string,
	executorsDB *sql.DB,
	parser parsing.SQLValidator,
	tableConstraints TableConstraints,
	fetchExtraBlockInfo bool,
) (chains.ChainStack, error) {
	store, err := system.New(dbURI, config.ChainID)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("failed initialize sqlstore: %s", err)
	}

	systemStore, err := sqlstoreimpl.NewInstrumentedSystemStore(config.ChainID, store)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("instrumenting system store: %s", err)
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
		Int64("chain_id", int64(config.ChainID)).
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

	ex, err := executor.NewExecutor(config.ChainID, executorsDB, parser, tableConstraints.MaxRowCount, acl)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("creating txn processor: %s", err)
	}
	chainAPIBackoff, err := time.ParseDuration(config.EventFeed.ChainAPIBackoff)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("parsing chain api backoff duration: %s", err)
	}
	newBlockPollFreq, err := time.ParseDuration(config.EventFeed.NewBlockPollFreq)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("parsing chain api backoff duration: %s", err)
	}
	efOpts := []eventfeed.Option{
		eventfeed.WithChainAPIBackoff(chainAPIBackoff),
		eventfeed.WithMinBlockDepth(config.EventFeed.MinBlockDepth),
		eventfeed.WithNewHeadPollFreq(newBlockPollFreq),
		eventfeed.WithEventPersistence(config.EventFeed.PersistEvents),
		eventfeed.WithFetchExtraBlockInformation(fetchExtraBlockInfo),
	}
	ef, err := efimpl.New(systemStore, config.ChainID, conn, scAddress, efOpts...)
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
		eventprocessor.WithHashCalcStep(config.HashCalculationStep),
	}
	ep, err := epimpl.New(parser, ex, ef, config.ChainID, epOpts...)
	if err != nil {
		return chains.ChainStack{}, fmt.Errorf("creating event processor: %s", err)
	}
	if err := ep.Start(); err != nil {
		return chains.ChainStack{}, fmt.Errorf("starting event processor: %s", err)
	}
	return chains.ChainStack{
		Store:                 systemStore,
		Registry:              registry,
		EventProcessor:        ep,
		AllowTransactionRelay: config.AllowTransactionRelay,
		Close: func(ctx context.Context) error {
			log.Info().Int64("chain_id", int64(config.ChainID)).Msg("closing stack...")
			defer log.Info().Int64("chain_id", int64(config.ChainID)).Msg("stack closed")

			ep.Stop()
			tracker.Close()
			conn.Close()
			if err := systemStore.Close(); err != nil {
				return fmt.Errorf("closing system store for chain_id %d: %s", config.ChainID, err)
			}
			return nil
		},
	}, nil
}

func configureTelemetry(
	dirPath string,
	chainStacks map[tableland.ChainID]chains.ChainStack,
	config TelemetryPublisherConfig,
) (moduleCloser, error) {
	var nodeID string
	var err error
	for chainID := range chainStacks {
		nodeID, err = chainStacks[chainID].Store.GetID(context.Background())
		if err != nil {
			return nil, fmt.Errorf("get node ID: %s", err)
		}
		log.Info().Str("node_id", nodeID).Msg("node info")
		break
	}

	// Wiring
	metricsDatabaseURL := fmt.Sprintf(
		"file://%s?_busy_timeout=5000&_foreign_keys=on&_journal_mode=WAL",
		path.Join(dirPath, "metrics.db"),
	)

	metricsStore, err := storage.New(metricsDatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("creating metrics store: %s", err)
	}

	var metricsPublisher *publisher.Publisher
	if config.Enabled {
		exporter, err := publisher.NewHTTPExporter(
			config.MetricsHubURL,
			config.MetricsHubAPIKey,
		)
		if err != nil {
			return nil, fmt.Errorf("creating metrics http exporter: %s", err)
		}
		publishingInterval, err := time.ParseDuration(config.PublishingInterval)
		if err != nil {
			return nil, fmt.Errorf("parsing publishing interval: %s", err)
		}
		metricsPublisher = publisher.NewPublisher(metricsStore, exporter, nodeID, publishingInterval)
		metricsPublisher.Start()
	}
	telemetry.SetMetricStore(metricsStore)

	// Collect binary information.
	ctx, cls := context.WithTimeout(context.Background(), time.Second)
	defer cls()
	if err := telemetry.Collect(ctx, buildinfo.GetSummary()); err != nil {
		return nil, fmt.Errorf("collect git summary: %s", err)
	}

	collectFrequency, err := time.ParseDuration(config.ChainStackCollectFrequency)
	if err != nil {
		return nil, fmt.Errorf("invalid chain stack collect frequency configuration: %s", err)
	}
	ctxChainStackCollector, clsChainStackCollector := context.WithCancel(context.Background())
	chainStackCollectorClosed := make(chan struct{})
	cc, err := chainscollector.New(chainStacks, collectFrequency)
	if err != nil {
		clsChainStackCollector()
		return nil, fmt.Errorf("configure chains collector: %s", err)
	}
	go func() {
		cc.Start(ctxChainStackCollector)
		close(chainStackCollectorClosed)
	}()

	// Module closing function
	return func(ctx context.Context) error {
		clsChainStackCollector()
		<-chainStackCollectorClosed

		if err := metricsStore.Close(); err != nil {
			return fmt.Errorf("closing metrics store: %s", err)
		}
		if config.Enabled {
			metricsPublisher.Close()
		}

		return nil
	}, nil
}

func restoreBackup(databaseURL string, backupURL string) error {
	restorer, err := restorer.NewBackupRestorer(backupURL, databaseURL)
	if err != nil {
		return fmt.Errorf("creating restorer: %s", err)
	}

	log.Info().Msg("starting backup restore")
	elapsedTime := time.Now()
	if err := restorer.Restore(); err != nil {
		return fmt.Errorf("backup restoration failed: %s", err)
	}
	log.Info().Float64("elapsed_time_seconds", time.Since(elapsedTime).Seconds()).Msg("backup restore finished")
	return nil
}

func createParser(queryConstraints QueryConstraints) (parsing.SQLValidator, error) {
	parserOpts := []parsing.Option{
		parsing.WithMaxReadQuerySize(queryConstraints.MaxReadQuerySize),
		parsing.WithMaxWriteQuerySize(queryConstraints.MaxWriteQuerySize),
	}

	parser, err := parserimpl.New([]string{
		"sqlite_",
		systemimpl.SystemTablesPrefix,
		systemimpl.RegistryTableName,
	}, parserOpts...)
	if err != nil {
		return nil, fmt.Errorf("new parser: %s", err)
	}

	parser, err = parserimpl.NewInstrumentedSQLValidator(parser)
	if err != nil {
		return nil, fmt.Errorf("instrumenting parser: %s", err)
	}

	return parser, nil
}

func createChainStacks(
	databaseURL string,
	parser parsing.SQLValidator,
	chainsConfig []ChainConfig,
	tableConstraintsConfig TableConstraints,
	fetchExtraBlockInfo bool,
) (map[tableland.ChainID]chains.ChainStack, moduleCloser, error) {
	executorsDB, err := otelsql.Open("sqlite3", databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("opening database: %s", err)
	}
	executorsDB.SetMaxOpenConns(1)
	attrs := append([]attribute.KeyValue{attribute.String("name", "executors")}, metrics.BaseAttrs...)
	if err := otelsql.RegisterDBStatsMetrics(
		executorsDB,
		otelsql.WithAttributes(attrs...)); err != nil {
		return nil, nil, fmt.Errorf("registering executors db stats: %s", err)
	}

	chainStacks := map[tableland.ChainID]chains.ChainStack{}
	for _, chainCfg := range chainsConfig {
		if _, ok := chainStacks[chainCfg.ChainID]; ok {
			return nil, nil, fmt.Errorf("duplicated chain id configuration for chain_id=%d", chainCfg.ChainID)
		}
		chainStack, err := createChainIDStack(
			chainCfg,
			databaseURL,
			executorsDB,
			parser,
			tableConstraintsConfig,
			fetchExtraBlockInfo)
		if err != nil {
			return nil, nil, fmt.Errorf("creating chain_id=%d stack: %s", chainCfg.ChainID, err)
		}
		chainStacks[chainCfg.ChainID] = chainStack
	}

	closeModule := func(ctx context.Context) error {
		// Close chains syncing.
		var wg sync.WaitGroup
		wg.Add(len(chainStacks))
		for chainID, stack := range chainStacks {
			go func(chainID tableland.ChainID, stack chains.ChainStack) {
				defer wg.Done()

				ctx, cls := context.WithTimeout(context.Background(), time.Second*15)
				defer cls()
				if err := stack.Close(ctx); err != nil {
					log.Error().Err(err).Int64("chain_id", int64(chainID)).Msg("finalizing chain stack")
				}
			}(chainID, stack)
		}
		wg.Wait()

		// Close Executor DB.
		if err := executorsDB.Close(); err != nil {
			return fmt.Errorf("closing executors db: %s", err)
		}
		return nil
	}

	return chainStacks, closeModule, nil
}

func createHTTPServer(
	httpConfig HTTPConfig,
	gatewayConfig GatewayConfig,
	parser parsing.SQLValidator,
	userStore *user.UserStore,
	chainStacks map[tableland.ChainID]chains.ChainStack,
) (moduleCloser, error) {
	rateLimInterval, err := time.ParseDuration(httpConfig.RateLimInterval)
	if err != nil {
		return nil, fmt.Errorf("parsing http ratelimiter interval: %s", err)
	}

	router := router.ConfiguredRouter(
		gatewayConfig.ExternalURIPrefix,
		gatewayConfig.MetadataRendererURI,
		gatewayConfig.MetadataRendererURI,
		httpConfig.MaxRequestPerInterval,
		rateLimInterval,
		parser,
		userStore,
		chainStacks,
	)

	server := &http.Server{
		Addr:         ":" + httpConfig.Port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  120 * time.Second,
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){},
		Handler:      router.Handler(),
	}

	if httpConfig.TLSCert != "" {
		tlsCert, err := base64.StdEncoding.DecodeString(httpConfig.TLSCert)
		if err != nil {
			return nil, fmt.Errorf("base64 decoding TLS certificate: %s", err)
		}
		tlsKey, err := base64.StdEncoding.DecodeString(httpConfig.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("base64 decoding TLS key: %s", err)
		}

		cert, err := tls.X509KeyPair(tlsCert, tlsKey)
		if err != nil {
			return nil, fmt.Errorf("parsing TLS certificate: %s", err)
		}
		server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
			CipherSuites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
			},
		}
		server.Addr = ":443"
	}

	go func() {
		if httpConfig.TLSCert != "" {
			if err := server.ListenAndServeTLS("", ""); err != nil {
				if err == http.ErrServerClosed {
					log.Info().Msg("https serve gracefully closed")
					return
				}
				log.Fatal().Err(err).Str("port", httpConfig.Port).Msg("couldn't not start HTTPS server")
			}
		} else {
			if err := server.ListenAndServe(); err != nil {
				if err == http.ErrServerClosed {
					log.Info().Msg("http server gracefully closed")
					return
				}
				log.Fatal().Err(err).Str("port", httpConfig.Port).Msg("couldn't start HTTP server")
			}
		}
	}()

	closeModule := func(ctx context.Context) error {
		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("closing HTTP server")
		}
		return nil
	}

	return closeModule, nil
}

func createBackuper(dirPath string, config BackupConfig) (moduleCloser, error) {
	backupScheduler, err := backup.NewScheduler(config.Frequency, backup.BackuperOptions{
		SourcePath: path.Join(dirPath, "database.db"),
		BackupDir:  path.Join(dirPath, config.Dir),
		Opts: []backup.Option{
			backup.WithCompression(config.EnableCompression),
			backup.WithVacuum(config.EnableVacuum),
			backup.WithPruning(config.Pruning.Enabled, config.Pruning.KeepFiles),
		},
	}, false)
	if err != nil {
		return nil, fmt.Errorf("creating backup scheduler: %s", err)
	}
	go backupScheduler.Run()

	closeModule := func(ctx context.Context) error {
		backupScheduler.Shutdown()
		return nil
	}

	return closeModule, nil
}
