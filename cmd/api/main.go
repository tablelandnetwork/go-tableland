package main

import (
	"context"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/internal/tableland/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/contract"
)

func main() {
	config := setupConfig()

	server := rpc.NewServer()

	ctx := context.Background()

	sqlstore, err := sqlstoreimpl.NewPostgres(ctx, config.DB.Host, config.DB.Port, config.DB.User, config.DB.Pass, config.DB.Name)
	if err != nil {
		panic(err)
	}
	defer sqlstore.Close()

	conn, err := ethclient.Dial(config.Registry.EthEndpoint)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	registry, err := contract.NewClient(conn, common.HexToAddress(config.Registry.ContractAddress))
	if err != nil {
		panic(err)
	}

	name, svc := getTablelandService(ctx, config, sqlstore, registry)
	server.RegisterName(name, svc)

	http.HandleFunc("/rpc", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Access-Control-Allow-Headers", "*")
		server.ServeHTTP(rw, r)
	})

	err = http.ListenAndServe(":"+config.HTTP.Port, nil)
	if err != nil {
		panic(err)
	}
}

func getTablelandService(ctx context.Context, conf *config, store sqlstore.SQLStore, registry *contract.Client) (string, tableland.Tableland) {
	switch conf.Impl {
	case "mesa":
		return tableland.ServiceName, impl.NewTablelandMesa(store, registry)
	case "mock":
		return tableland.ServiceName, new(impl.TablelandMock)

	}
	return tableland.ServiceName, new(impl.TablelandMock)
}
