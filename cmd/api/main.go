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

type moduleClose func(ctx context.Context) error

func main() {
	config, dirPath := setupConfig()
	logging.SetupLogger(buildinfo.GitCommit, config.Log.Debug, config.Log.Human)

	// Validator instrumentation configuration.
	if err := metrics.SetupInstrumentation(":" + config.Metrics.Port); err != nil {
		log.Fatal().
			Err(err).
			Str("port", config.Metrics.Port).
			Msg("could not setup instrumentation")
	}

	databaseURL := fmt.Sprintf(
		"file://%s?_busy_timeout=5000&_foreign_keys=on&_journal_mode=WAL",
		path.Join(dirPath, "database.db"),
	)

	if config.BootstrapBackupURL != "" {
		restorer, err := restorer.NewBackupRestorer(config.BootstrapBackupURL, databaseURL)
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("creating restorer")
		}

		log.Info().Msg("starting backup restore")
		elapsedTime := time.Now()
		if err := restorer.Restore(); err != nil {
			log.Fatal().
				Err(err).
				Msg("backup restoration failed")
		}
		log.Info().Float64("elapsed_time_seconds", time.Since(elapsedTime).Seconds()).Msg("backup restore finished")
	}

	parserOpts := []parsing.Option{
		parsing.WithMaxReadQuerySize(config.QueryConstraints.MaxReadQuerySize),
		parsing.WithMaxWriteQuerySize(config.QueryConstraints.MaxWriteQuerySize),
	}

	parser, err := parserimpl.New([]string{
		"sqlite_",
		systemimpl.SystemTablesPrefix,
		systemimpl.RegistryTableName,
	}, parserOpts...)
	if err != nil {
		log.Fatal().Err(err).Msg("new parser")
	}

	parser, err = parserimpl.NewInstrumentedSQLValidator(parser)
	if err != nil {
		log.Fatal().Err(err).Msg("instrumenting sql validator")
	}

	executorsDB, err := otelsql.Open("sqlite3", databaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("creating executors db")
	}
	executorsDB.SetMaxOpenConns(1)
	if err := otelsql.RegisterDBStatsMetrics(
		executorsDB,
		otelsql.WithAttributes(attribute.String("name", "executors"))); err != nil {
		log.Fatal().Err(err).Msg("registering executors db stats")
	}

	chainStacks := map[tableland.ChainID]chains.ChainStack{}
	for _, chainCfg := range config.Chains {
		if _, ok := chainStacks[chainCfg.ChainID]; ok {
			log.Fatal().Int64("chain_id", int64(chainCfg.ChainID)).Msg("chain id configuration is duplicated")
		}
		chainStack, err := createChainIDStack(
			chainCfg,
			databaseURL,
			executorsDB,
			parser,
			config.TableConstraints,
			config.Analytics.FetchExtraBlockInfo)
		if err != nil {
			log.Fatal().Int64("chain_id", int64(chainCfg.ChainID)).Err(err).Msg("spinning up chain stack")
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
		config.Gateway.MetadataRendererURI,
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
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){},
		Handler:      router.Handler(),
	}

	go func() {
		if config.HTTP.TLSCert != "" {
			tlsCert, err := base64.StdEncoding.DecodeString(config.HTTP.TLSCert)
			if err != nil {
				log.Fatal().
					Err(err).
					Msg("could not base64 decode tls cert")
			}
			tlsKey, err := base64.StdEncoding.DecodeString(config.HTTP.TLSKey)
			if err != nil {
				log.Fatal().
					Err(err).
					Msg("could not base64 decode tls cert")
			}

			cert, err := tls.X509KeyPair(tlsCert, tlsKey)
			if err != nil {
				log.Fatal().
					Err(err).
					Msg("parsing tls certificate")
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
			if err := server.ListenAndServeTLS("", ""); err != nil {
				if err == http.ErrServerClosed {
					log.Info().Msg("https serve gracefully closed")
					return
				}
				log.Fatal().
					Err(err).
					Msg("could not start https server")
			}
		} else {
			if err := server.ListenAndServe(); err != nil {
				if err == http.ErrServerClosed {
					log.Info().Msg("http server gracefully closed")
					return
				}
				log.Fatal().
					Err(err).
					Str("port", config.HTTP.Port).
					Msg("could not start http server")
			}
		}
	}()

	var backupScheduler *backup.Scheduler
	if config.Backup.Enabled {
		backupScheduler, err = backup.NewScheduler(config.Backup.Frequency, backup.BackuperOptions{
			SourcePath: path.Join(dirPath, "database.db"),
			BackupDir:  path.Join(dirPath, config.Backup.Dir),
			Opts: []backup.Option{
				backup.WithCompression(config.Backup.EnableCompression),
				backup.WithVacuum(config.Backup.EnableVacuum),
				backup.WithPruning(config.Backup.Pruning.Enabled, config.Backup.Pruning.KeepFiles),
			},
		}, false)
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("could not instantiate backup scheduler")
		}
		go backupScheduler.Run()
	}

	nodeID, err := chainStacks[config.Chains[0].ChainID].Store.GetID(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("getting node id")
	}

	log.Info().Str("node_id", nodeID).Msg("node info")

	closeTelemetryModule, err := configureTelemetry(nodeID, dirPath, chainStacks, config.TelemetryPublisher)
	if err != nil {
		log.Fatal().Err(err).Msg("configuring telemetry")
	}

	cli.HandleInterrupt(func() {
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

		ctx, cls := context.WithTimeout(context.Background(), time.Second*10)
		defer cls()
		if err := server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("shutting down http server")
		}

		if config.Backup.Enabled {
			backupScheduler.Shutdown()
		}

		if err := userStore.Close(); err != nil {
			log.Error().Err(err).Msg("closing user store")
		}

		if err := executorsDB.Close(); err != nil {
			log.Error().Err(err).Msg("closing executors db")
		}

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
				log.Error().Int64("chain_id", int64(config.ChainID)).Err(err).Msg("closing system store")
			}
			return nil
		},
	}, nil
}

func configureTelemetry(
	nodeID string,
	dirPath string,
	chainStacks map[tableland.ChainID]chains.ChainStack,
	config TelemetryPublisherConfig,
) (moduleClose, error) {
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
