/*
 * Tableland Validator - OpenAPI 3.0
 *
 * In Tableland, Validators are the execution unit/actors of the protocol. They have the following responsibilities: - Listen to on-chain events to materialize Tableland-compliant SQL queries in a database engine (currently, SQLite by default). - Serve read-queries (e.g: SELECT * FROM foo_69_1) to the external world. - Serve state queries (e.g. list tables, get receipts, etc) to the external world.  In the 1.0.0 release of the Tableland Validator API, we've switched to a design first approach! You can now help us improve the API whether it's by making changes to the definition itself or to the code. That way, with time, we can improve the API in general, and expose some of the new features in OAS3.
 *
 * API version: 1.0.0
 * Contact: carson@textile.io
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package apiv1

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type Routes []Route

func NewRouter() *chi.Mux {
	router := chi.NewRouter()
	for _, route := range routes {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = Logger(handler, route.Name)

		router.Method(route.Method, route.Pattern, handler)
	}

	return router
}

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World!")
}

var routes = Routes{
	Route{
		"Index",
		"GET",
		"/api/v1/",
		Index,
	},

	Route{
		"Health",
		strings.ToUpper("Get"),
		"/api/v1/health",
		Health,
	},

	Route{
		"QueryByStatement",
		strings.ToUpper("Get"),
		"/api/v1/query",
		QueryByStatement,
	},

	Route{
		"ReceiptByTransactionHash",
		strings.ToUpper("Get"),
		"/api/v1/receipt/{chainId}/{transactionHash}",
		ReceiptByTransactionHash,
	},

	Route{
		"GetTableById",
		strings.ToUpper("Get"),
		"/api/v1/tables/{chainId}/{tableId}",
		GetTableById,
	},

	Route{
		"Version",
		strings.ToUpper("Get"),
		"/api/v1/version",
		Version,
	},
}
