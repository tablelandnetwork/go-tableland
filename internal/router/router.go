package router

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"
	"github.com/textileio/go-tableland/internal/chains"
	"github.com/textileio/go-tableland/internal/router/controllers"
	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
	"github.com/textileio/go-tableland/internal/router/controllers/legacy"
	"github.com/textileio/go-tableland/internal/router/middlewares"
	systemimpl "github.com/textileio/go-tableland/internal/system/impl"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/internal/tableland/impl"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/user"
)

// ConfiguredRouter returns a fully configured Router that can be used as an http handler.
func ConfiguredRouter(
	extURLPrefix string,
	metadataRendererURI string,
	animationRendererURI string,
	maxRPI uint64,
	rateLimInterval time.Duration,
	parser parsing.SQLValidator,
	userStore *user.UserStore,
	chainStacks map[tableland.ChainID]chains.ChainStack,
) (*Router, error) {
	instrUserStore, err := sqlstoreimpl.NewInstrumentedUserStore(userStore)
	if err != nil {
		return nil, fmt.Errorf("creating instrumented user store: %s", err)
	}

	mesaService := impl.NewTablelandMesa(parser, instrUserStore, chainStacks)
	mesaService, err = impl.NewInstrumentedTablelandMesa(mesaService)
	if err != nil {
		return nil, fmt.Errorf("instrumenting mesa: %s", err)
	}
	rpcService := legacy.NewRPCService(mesaService)
	server := rpc.NewServer()
	if err := server.RegisterName("tableland", rpcService); err != nil {
		return nil, fmt.Errorf("failed to register a json-rpc service: %s", err)
	}
	userController := controllers.NewUserController(mesaService)

	infraController := controllers.NewInfraController()

	stores := make(map[tableland.ChainID]sqlstore.SystemStore, len(chainStacks))
	for chainID, stack := range chainStacks {
		stores[chainID] = stack.Store
	}
	sysStore, err := systemimpl.NewSystemSQLStoreService(stores, extURLPrefix, metadataRendererURI, animationRendererURI)
	if err != nil {
		return nil, fmt.Errorf("creating system store: %s", err)
	}
	systemService, err := systemimpl.NewInstrumentedSystemSQLStoreService(sysStore)
	if err != nil {
		return nil, fmt.Errorf("instrumenting system sql store: %s", err)
	}
	systemController := controllers.NewSystemController(systemService)

	// General router configuration.
	router := newRouter()
	router.use(middlewares.CORS, middlewares.TraceID)

	// RPC configuration.
	cfg := middlewares.RateLimiterConfig{
		Default: middlewares.RateLimiterRouteConfig{
			MaxRPI:   maxRPI,
			Interval: rateLimInterval,
		},
		JSONRPCRoute: "/rpc",
	}
	rateLim, err := middlewares.RateLimitController(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating rate limit controller middleware: %s", err)
	}

	if err := configureLegacyAPI(router, server, rateLim, systemController, userController, infraController); err != nil {
		return nil, fmt.Errorf("configuring legacy API: %s", err)
	}
	if err := configureAPIv1(router, rateLim, systemController, userController, infraController); err != nil {
		return nil, fmt.Errorf("configuring API v1: %s", err)
	}

	return router, nil
}

func configureLegacyAPI(
	router *Router,
	server *rpc.Server,
	rateLim mux.MiddlewareFunc,
	systemController *controllers.SystemController,
	userController *controllers.UserController,
	infraController *controllers.InfraController,
) error {
	router.post("/rpc", func(rw http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(rw, r)
	}, middlewares.WithLogging, middlewares.OtelHTTP("rpc"), middlewares.Authentication, rateLim)

	// Gateway configuration.
	router.get("/chain/{chainId}/tables/{tableId}", systemController.GetTable, middlewares.WithLogging, middlewares.OtelHTTP("GetTable"), middlewares.RESTChainID, rateLim)                                        // nolint
	router.get("/chain/{chainId}/tables/{id}/{key}/{value}", userController.GetTableRow, middlewares.WithLogging, middlewares.OtelHTTP("GetTableRow"), middlewares.RESTChainID, rateLim)                           // nolint
	router.get("/chain/{chainId}/tables/controller/{address}", systemController.GetTablesByController, middlewares.WithLogging, middlewares.OtelHTTP("GetTablesByController"), middlewares.RESTChainID, rateLim)   // nolint
	router.get("/chain/{chainId}/tables/structure/{hash}", systemController.GetTablesByStructureHash, middlewares.WithLogging, middlewares.OtelHTTP("GetTablesByStructureHash"), middlewares.RESTChainID, rateLim) // nolint
	router.get("/schema/{table_name}", systemController.GetSchemaByTableName, middlewares.WithLogging, middlewares.OtelHTTP("GetSchemaFromTableName"), rateLim)                                                    // nolint

	router.get("/query", userController.GetTableQuery, middlewares.WithLogging, middlewares.OtelHTTP("GetTableQuery"), rateLim) // nolint
	router.get("/version", infraController.Version, middlewares.WithLogging, middlewares.OtelHTTP("Version"), rateLim)          // nolint

	// Health endpoint configuration.
	router.get("/healthz", controllers.HealthHandler)
	router.get("/health", controllers.HealthHandler)

	return nil
}

func configureAPIv1(
	router *Router,
	rateLim mux.MiddlewareFunc,
	systemController *controllers.SystemController,
	userController *controllers.UserController,
	infraController *controllers.InfraController,
) error {
	handlers := map[string]http.HandlerFunc{
		"QueryFromQuery":   userController.GetTableQuery,
		"ReceiptByTxnHash": systemController.GetTxnHash, // TODO(jsign): do it.
		"GetTableById":     systemController.GetTable,   // TODO(jsign): verify output.
		"Version":          infraController.Version,
		"Health":           controllers.HealthHandler,
	}

	var specRoutesCount int
	if err := apiv1.NewRouter().Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		handle, ok := handlers[route.GetName()]
		if !ok {
			return fmt.Errorf("route with name %s not found in handler", route.GetName())
		}
		pathTemplate, err := route.GetPathTemplate()
		if err != nil {
			return fmt.Errorf("get path template: %s", err)
		}
		router.get(
			pathTemplate,
			handle,
			middlewares.WithLogging, middlewares.OtelHTTP(route.GetName()), middlewares.RESTChainID, rateLim)
		specRoutesCount++
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
