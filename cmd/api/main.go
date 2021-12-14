package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/textileio/go-tableland/cmd/api/controllers"
	"github.com/textileio/go-tableland/internal/system"
	systemimpl "github.com/textileio/go-tableland/internal/system/impl"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/internal/tableland/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
)

var (
	systemService    system.SystemService          = systemimpl.NewSystemMockService()
	systemController *controllers.SystemController = controllers.NewSystemController(systemService)
)

func main() {
	config := setupConfig()

	server := rpc.NewServer()

	ctx := context.Background()

	databaseUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&timezone=UTC", config.DB.User, config.DB.Pass, config.DB.Host, config.DB.Port, config.DB.Name)
	sqlstore, err := sqlstoreimpl.New(ctx, databaseUrl)
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

	name, svc := getTablelandService(config, sqlstore, registry)
	server.RegisterName(name, svc)

	router := NewRouter()
	router.Post("/rpc", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Access-Control-Allow-Headers", "*")
		server.ServeHTTP(rw, r)
	})
	router.Get("/tables/{uuid}", systemController.GetTables)

	err = router.Serve(":" + config.HTTP.Port)
	if err != nil {
		panic(err)
	}
}

func getTablelandService(conf *config, store sqlstore.SQLStore, registry *ethereum.Client) (string, tableland.Tableland) {
	switch conf.Impl {
	case "mesa":
		return tableland.ServiceName, impl.NewTablelandMesa(store, registry)
	case "mock":
		return tableland.ServiceName, new(impl.TablelandMock)

	}
	return tableland.ServiceName, new(impl.TablelandMock)
}
