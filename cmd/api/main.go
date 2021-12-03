package main

import (
	"context"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/internal/tableland/impl"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/erc1155"
)

func main() {
	config := setupConfig()

	server := rpc.NewServer()

	ctx := context.Background()
	name, svc := getTablelandService(ctx, config)
	server.RegisterName(name, svc)

	http.HandleFunc("/rpc", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Access-Control-Allow-Headers", "*")
		server.ServeHTTP(rw, r)
	})

	err := http.ListenAndServe(":"+config.HTTP.Port, nil)
	if err != nil {
		panic(err)
	}
}

func getTablelandService(ctx context.Context, conf *config) (string, tableland.Tableland) {
	switch conf.Impl {
	case "mesa":
		sqlstore, err := sqlstoreimpl.NewPostgres(ctx, conf.DB.Host, conf.DB.Port, conf.DB.User, conf.DB.Pass, conf.DB.Name)
		if err != nil {
			panic(err)
		}
		registry, err := erc1155.NewClient(conf.Registry.EthEndpoint, common.HexToAddress(conf.Registry.ContractAddress))
		if err != nil {
			panic(err)
		}
		return tableland.ServiceName, impl.NewTablelandMesa(sqlstore, registry)

	case "mock":
		return tableland.ServiceName, new(impl.TablelandMock)

	}
	return tableland.ServiceName, new(impl.TablelandMock)
}
