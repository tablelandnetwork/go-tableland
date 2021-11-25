package main

import (
	"net/http"

	"github.com/brunocalza/go-tableland/internal/tableland"
	"github.com/brunocalza/go-tableland/internal/tableland/impl"
	"github.com/ethereum/go-ethereum/rpc"
)

func main() {
	config := SetupConfig()

	server := rpc.NewServer()

	name, svc := getTablelandService(config)
	server.RegisterName(name, svc)

	http.HandleFunc("/rpc", func(rw http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(rw, r)
	})

	err := http.ListenAndServe(":"+config.HTTP.Port, nil)
	if err != nil {
		panic(err)
	}
}

func getTablelandService(config *config) (string, tableland.Tableland) {
	switch config.Impl {
	case "mesa":
		fallthrough // mesa is not implemented yet
	case "mock":
		return tableland.ServiceName, new(impl.TablelandMock)

	}
	return tableland.ServiceName, new(impl.TablelandMock)
}
