package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/textileio/go-tableland/cmd/api/controllers"
	"github.com/textileio/go-tableland/cmd/api/middlewares"
	systemimpl "github.com/textileio/go-tableland/internal/system/impl"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/internal/tableland/impl"
	"github.com/textileio/go-tableland/pkg/metrics"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
)

func main() {
	config := setupConfig()

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
		panic(err)
	}
	defer sqlstore.Close()

	conn, err := ethclient.Dial(config.Registry.EthEndpoint)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	registry, err := ethereum.NewClient(conn, common.HexToAddress(config.Registry.ContractAddress))
	if err != nil {
		panic(err)
	}

	sqlstore = sqlstoreimpl.NewInstrumentedSQLStorePGX(sqlstore)

	svc := getTablelandService(config, sqlstore, registry)
	if err := server.RegisterName("tableland", svc); err != nil {
		panic(err)
	}

	systemService := systemimpl.NewInstrumentedSystemSQLStoreService(systemimpl.NewSystemSQLStoreService(sqlstore))
	systemController := controllers.NewSystemController(systemService)

	router := newRouter()
	router.Use(middlewares.CORS)
	router.Post("/rpc", func(rw http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(rw, r)
	}, middlewares.Authentication)

	router.Get("/tables/{uuid}", systemController.GetTable)
	router.Get("/tables/controller/{address}", systemController.GetTablesByController)
	router.Get("/healthz", healthHandler)
	router.Get("/health", healthHandler)

	if err := metrics.SetupInstrumentation(":" + config.Metrics.Port); err != nil {
		panic(err)
	}

	if err := router.Serve(":" + config.HTTP.Port); err != nil {
		panic(err)
	}
}

func getTablelandService(
	conf *config,
	store sqlstore.SQLStore,
	registry *ethereum.Client,
) tableland.Tableland {
	switch conf.Impl {
	case "mesa":
		mesa := impl.NewTablelandMesa(store, registry)
		return impl.NewInstrumentedTablelandMesa(mesa)
	case "mock":
		return new(impl.TablelandMock)
	}
	return new(impl.TablelandMock)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
