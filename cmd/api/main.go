package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/buildinfo"
	"github.com/textileio/go-tableland/cmd/api/controllers"
	"github.com/textileio/go-tableland/cmd/api/middlewares"
	systemimpl "github.com/textileio/go-tableland/internal/system/impl"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/internal/tableland/impl"
	"github.com/textileio/go-tableland/pkg/metrics"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/txn"
	txnimpl "github.com/textileio/go-tableland/pkg/txn/impl"
)

func main() {
	config := setupConfig()
	setupLogger(buildinfo.GitCommit, config.Log.Debug, config.Log.Human)

	server := rpc.NewServer()
	ctx := context.Background()

	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&timezone=UTC",
		config.DB.User,
		config.DB.Pass,
		config.DB.Host,
		config.DB.Port,
		config.DB.Name,
	)
	sqlstore, err := sqlstoreimpl.New(ctx, databaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed initialize sqlstore")
	}
	defer sqlstore.Close()

	conn, err := ethclient.Dial(config.Registry.EthEndpoint)
	if err != nil {
		log.Fatal().
			Err(err).
			Str("ethEndpoint", config.Registry.EthEndpoint).
			Msg("failed to connect to ethereum endpoint")
	}
	defer conn.Close()

	registry, err := ethereum.NewClient(conn, common.HexToAddress(config.Registry.ContractAddress))
	if err != nil {
		log.Fatal().
			Err(err).
			Str("contractAddress", config.Registry.ContractAddress).
			Msg("failed to create new ethereum client")
	}

	sqlstore = sqlstoreimpl.NewInstrumentedSQLStorePGX(sqlstore)
	parser := parserimpl.NewInstrumentedSQLValidator(parserimpl.New(systemimpl.SystemTablesPrefix))

	txnp, err := txnimpl.NewTxnProcessor(databaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("creating txn processor")
	}

	svc := getTablelandService(config, sqlstore, registry, parser, txnp)
	if err := server.RegisterName("tableland", svc); err != nil {
		log.Fatal().
			Err(err).
			Msg("failed to register a json-rpc service")
	}

	systemService := systemimpl.NewInstrumentedSystemSQLStoreService(systemimpl.NewSystemSQLStoreService(sqlstore))
	systemController := controllers.NewSystemController(systemService)

	router := newRouter()
	router.Use(middlewares.CORS, middlewares.TraceID)
	router.Post("/rpc", func(rw http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(rw, r)
	}, middlewares.Authentication, middlewares.VerifyController, middlewares.OtelHTTP("rpc"))

	router.Get("/tables/{uuid}", systemController.GetTable, middlewares.OtelHTTP("GetTable"))
	router.Get("/tables/controller/{address}", systemController.GetTablesByController, middlewares.OtelHTTP("GetTablesByController")) //nolint

	router.Get("/healthz", healthHandler)
	router.Get("/health", healthHandler)

	if config.AdminAPI.Password == "" {
		log.Warn().
			Msg("no admin api password set")
	}
	basicAuth := middlewares.BasicAuth(config.AdminAPI.Username, config.AdminAPI.Password)
	router.Post("/authorized-addresses", systemController.Authorize, basicAuth, middlewares.OtelHTTP("Authorize"))
	router.Get("/authorized-addresses/{address}", systemController.IsAuthorized, basicAuth, middlewares.OtelHTTP("IsAuthorized")) //nolint
	router.Delete("/authorized-addresses/{address}", systemController.Revoke, basicAuth, middlewares.OtelHTTP("Revoke"))
	router.Get("/authorized-addresses/{address}/record", systemController.GetAuthorizationRecord, basicAuth, middlewares.OtelHTTP("GetAuthorizationRecord")) //nolint
	router.Get("/authorized-addresses", systemController.ListAuthorized, basicAuth, middlewares.OtelHTTP("ListAuthorized"))

	if err := metrics.SetupInstrumentation(":" + config.Metrics.Port); err != nil {
		log.Fatal().
			Err(err).
			Str("port", config.Metrics.Port).
			Msg("could not setup instrumentation")
	}

	if err := router.Serve(":" + config.HTTP.Port); err != nil {
		log.Fatal().
			Err(err).
			Str("port", config.HTTP.Port).
			Msg("could not start server")
	}
}

func getTablelandService(
	conf *config,
	store sqlstore.SQLStore,
	registry *ethereum.Client,
	parser parsing.SQLValidator,
	txnp txn.TxnProcessor,
) tableland.Tableland {
	switch conf.Impl {
	case "mesa":
		mesa := impl.NewTablelandMesa(store, registry, parser, txnp)
		return impl.NewInstrumentedTablelandMesa(mesa)
	case "mock":
		return new(impl.TablelandMock)
	}
	return new(impl.TablelandMock)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
