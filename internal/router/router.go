package router

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"
	"github.com/textileio/go-tableland/internal/router/controllers"
	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
	"github.com/textileio/go-tableland/internal/router/controllers/legacy"
	"github.com/textileio/go-tableland/internal/router/middlewares"
	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/internal/tableland"
)

// ConfiguredRouter returns a fully configured Router that can be used as an http handler.
func ConfiguredRouter(
	tableland tableland.Tableland,
	systemService system.SystemService,
	maxRPI uint64,
	rateLimInterval time.Duration,
	supportedChainIDs []tableland.ChainID,
) (*Router, error) {
	rpcService := legacy.NewRPCService(tableland)
	server := rpc.NewServer()
	if err := server.RegisterName("tableland", rpcService); err != nil {
		return nil, fmt.Errorf("failed to register a json-rpc service: %s", err)
	}
	userCtrl := controllers.NewUserController(tableland)

	systemCtrl := controllers.NewSystemController(systemService)

	// General router configuration.
	router := newRouter()
	router.use(middlewares.CORS, middlewares.TraceID)

	cfg := middlewares.RateLimiterConfig{
		Default: middlewares.RateLimiterRouteConfig{
			MaxRPI:   maxRPI,
			Interval: rateLimInterval,
		},
		JSONRPCRoute: "/rpc", // TODO(json-rpc): remove this feature in the rate-limiter when we drop support.
	}
	rateLim, err := middlewares.RateLimitController(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating rate limit controller middleware: %s", err)
	}

	// TODO(json-rpc): remove this when dropping support.
	// APIs Legacy (REST + JSON-RPC)
	configureLegacyRoutes(router, server, supportedChainIDs, rateLim, systemCtrl, userCtrl)

	// APIs V1
	if err := configureAPIV1Routes(router, supportedChainIDs, rateLim, systemCtrl, userCtrl); err != nil {
		return nil, fmt.Errorf("configuring API v1: %s", err)
	}

	return router, nil
}

func configureLegacyRoutes(
	router *Router,
	server *rpc.Server,
	supportedChainIDs []tableland.ChainID,
	rateLim mux.MiddlewareFunc,
	systemCtrl *controllers.SystemController,
	userCtrl *controllers.UserController,
) {
	router.post("/rpc", func(rw http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(rw, r)
	}, middlewares.WithLogging, middlewares.OtelHTTP("rpc"), middlewares.Authentication, rateLim)

	// Gateway configuration.
	router.get("/chain/{chainId}/tables/{tableId}", systemCtrl.GetTable, middlewares.WithLogging, middlewares.OtelHTTP("GetTable"), middlewares.RESTChainID(supportedChainIDs), rateLim)                                        // nolint
	router.get("/chain/{chainId}/tables/{id}/{key}/{value}", userCtrl.GetTableRow, middlewares.WithLogging, middlewares.OtelHTTP("GetTableRow"), middlewares.RESTChainID(supportedChainIDs), rateLim)                           // nolint
	router.get("/chain/{chainId}/tables/controller/{address}", systemCtrl.GetTablesByController, middlewares.WithLogging, middlewares.OtelHTTP("GetTablesByController"), middlewares.RESTChainID(supportedChainIDs), rateLim)   // nolint
	router.get("/chain/{chainId}/tables/structure/{hash}", systemCtrl.GetTablesByStructureHash, middlewares.WithLogging, middlewares.OtelHTTP("GetTablesByStructureHash"), middlewares.RESTChainID(supportedChainIDs), rateLim) // nolint
	router.get("/schema/{table_name}", systemCtrl.GetSchemaByTableName, middlewares.WithLogging, middlewares.OtelHTTP("GetSchemaFromTableName"), rateLim)                                                                       // nolint

	router.get("/query", userCtrl.GetTableQuery, middlewares.WithLogging, middlewares.OtelHTTP("GetTableQuery"), rateLim) // nolint
	router.get("/version", userCtrl.Version, middlewares.WithLogging, middlewares.OtelHTTP("Version"), rateLim)           // nolint

	// Health endpoint configuration.
	router.get("/healthz", controllers.HealthHandler)
	router.get("/health", controllers.HealthHandler)
}

func configureAPIV1Routes(
	router *Router,
	supportedChainIDs []tableland.ChainID,
	rateLim mux.MiddlewareFunc,
	systemCtrl *controllers.SystemController,
	userCtrl *controllers.UserController,
) error {
	handlers := map[string]struct {
		handler     http.HandlerFunc
		middlewares []mux.MiddlewareFunc
	}{
		"QueryByStatement": {
			userCtrl.GetTableQuery,
			[]mux.MiddlewareFunc{middlewares.WithLogging, rateLim},
		},
		"ReceiptByTransactionHash": {
			systemCtrl.GetReceiptByTransactionHash,
			[]mux.MiddlewareFunc{middlewares.WithLogging, middlewares.RESTChainID(supportedChainIDs), rateLim},
		},
		"GetTableById": {
			systemCtrl.GetTable,
			[]mux.MiddlewareFunc{middlewares.WithLogging, middlewares.RESTChainID(supportedChainIDs), rateLim},
		},
		"Version": {
			userCtrl.Version,
			[]mux.MiddlewareFunc{middlewares.WithLogging, rateLim},
		},
		"Health": {
			controllers.HealthHandler,
			[]mux.MiddlewareFunc{middlewares.WithLogging, rateLim},
		},
	}

	var specRoutesCount int
	if err := apiv1.NewRouter().Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		routeName := route.GetName()
		// Ignore the "Index" API that OpenAPI 3.0 code generator always create for the base `/` route.
		if routeName == "Index" {
			return nil
		}

		specRoutesCount++
		endpoint, ok := handlers[routeName]
		if !ok {
			return fmt.Errorf("route with name %s not found in handler", routeName)
		}
		pathTemplate, err := route.GetPathTemplate()
		if err != nil {
			return fmt.Errorf("get path template: %s", err)
		}

		router.get(
			pathTemplate,
			endpoint.handler,
			append(endpoint.middlewares, middlewares.OtelHTTP(routeName))...,
		)
		return nil
	}); err != nil {
		return fmt.Errorf("configuring api v1 router: %s", err)
	}
	if specRoutesCount != len(handlers) {
		return fmt.Errorf("the spec has less defined routes than expected handlers to be used")
	}

	return nil
}

// Router provides a nice api around mux.Router.
type Router struct {
	r *mux.Router
}

// newRouter is a Mux HTTP router constructor.
func newRouter() *Router {
	r := mux.NewRouter()
	r.PathPrefix("/").Methods(http.MethodOptions) // accept OPTIONS on all routes and do nothing
	return &Router{r: r}
}

// get creates a subroute on the specified URI that only accepts GET. You can provide specific middlewares.
func (r *Router) get(uri string, f http.HandlerFunc, mid ...mux.MiddlewareFunc) {
	sub := r.r.Path(uri).Subrouter()
	sub.HandleFunc("", f).Methods(http.MethodGet)
	sub.Use(mid...)
}

// post creates a subroute on the specified URI that only accepts POST. You can provide specific middlewares.
func (r *Router) post(uri string, f func(http.ResponseWriter, *http.Request), mid ...mux.MiddlewareFunc) {
	sub := r.r.Path(uri).Subrouter()
	sub.HandleFunc("", f).Methods(http.MethodPost)
	sub.Use(mid...)
}

// use adds middlewares to all routes. Should be used when a middleware should be execute all all routes (e.g. CORS).
func (r *Router) use(mid ...mux.MiddlewareFunc) {
	r.r.Use(mid...)
}

// Handler returns the configured router http handler.
func (r *Router) Handler() http.Handler {
	return r.r
}
