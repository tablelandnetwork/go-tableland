package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/jackc/pgx/v4"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/internal/tableland/impl"
)

func main() {
	config := setupConfig()
	testDatabaseConnection(config)

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

func testDatabaseConnection(conf *config) {
	databaseUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&timezone=UTC", conf.DB.User, conf.DB.Pass, conf.DB.Host, conf.DB.Port, conf.DB.Name)

	conn, err := pgx.Connect(context.Background(), databaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	var s string
	err = conn.QueryRow(context.Background(), "select 'Hello Database!'").Scan(&s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(s)

	defer conn.Close(context.Background())
}
