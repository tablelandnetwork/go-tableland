package router

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"
	"github.com/textileio/go-tableland/internal/chains"
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
	userController := legacy.NewUserController(mesaService)

	server := rpc.NewServer()
	if err := server.RegisterName("tableland", rpcService); err != nil {
		return nil, fmt.Errorf("failed to register a json-rpc service: %s", err)
	}
	infraController := legacy.NewInfraController()

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
	systemController := legacy.NewSystemController(systemService)

	// General router configuration.
	router := NewRouter()
	router.Use(middlewares.CORS, middlewares.TraceID)

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

	router.Post("/rpc", func(rw http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(rw, r)
	}, middlewares.WithLogging, middlewares.OtelHTTP("rpc"), middlewares.Authentication, rateLim)

	// Gateway configuration.
	router.Get("/chain/{chainID}/tables/{id}", systemController.GetTable, middlewares.WithLogging, middlewares.OtelHTTP("GetTable"), middlewares.RESTChainID, rateLim)                                             // nolint
	router.Get("/chain/{chainID}/tables/{id}/{key}/{value}", userController.GetTableRow, middlewares.WithLogging, middlewares.OtelHTTP("GetTableRow"), middlewares.RESTChainID, rateLim)                           // nolint
	router.Get("/chain/{chainID}/tables/controller/{address}", systemController.GetTablesByController, middlewares.WithLogging, middlewares.OtelHTTP("GetTablesByController"), middlewares.RESTChainID, rateLim)   // nolint
	router.Get("/chain/{chainID}/tables/structure/{hash}", systemController.GetTablesByStructureHash, middlewares.WithLogging, middlewares.OtelHTTP("GetTablesByStructureHash"), middlewares.RESTChainID, rateLim) // nolint
	router.Get("/schema/{table_name}", systemController.GetSchemaByTableName, middlewares.WithLogging, middlewares.OtelHTTP("GetSchemaFromTableName"), rateLim)                                                    // nolint

	router.Get("/query", userController.GetTableQuery, middlewares.WithLogging, middlewares.OtelHTTP("GetTableQuery"), rateLim) // nolint
	router.Get("/version", infraController.Version, middlewares.WithLogging, middlewares.OtelHTTP("Version"), rateLim)          // nolint

	// Health endpoint configuration.
	router.Get("/healthz", healthHandler)
	router.Get("/health", healthHandler)

	return router, nil
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Router provides a nice api around mux.Router.
type Router struct {
	r *mux.Router
}

// NewRouter is a Mux HTTP router constructor.
func NewRouter() *Router {
	r := mux.NewRouter()
	r.PathPrefix("/").Methods(http.MethodOptions) // accept OPTIONS on all routes and do nothing
	return &Router{r: r}
}

// Get creates a subroute on the specified URI that only accepts GET. You can provide specific middlewares.
func (r *Router) Get(uri string, f func(http.ResponseWriter, *http.Request), mid ...mux.MiddlewareFunc) {
	sub := r.r.Path(uri).Subrouter()
	sub.HandleFunc("", f).Methods(http.MethodGet)
	sub.Use(mid...)
}

// Post creates a subroute on the specified URI that only accepts POST. You can provide specific middlewares.
func (r *Router) Post(uri string, f func(http.ResponseWriter, *http.Request), mid ...mux.MiddlewareFunc) {
	sub := r.r.Path(uri).Subrouter()
	sub.HandleFunc("", f).Methods(http.MethodPost)
	sub.Use(mid...)
}

// Delete creates a subroute on the specified URI that only accepts DELETE. You can provide specific middlewares.
func (r *Router) Delete(uri string, f func(http.ResponseWriter, *http.Request), mid ...mux.MiddlewareFunc) {
	sub := r.r.Path(uri).Subrouter()
	sub.HandleFunc("", f).Methods(http.MethodDelete)
	sub.Use(mid...)
}

// Use adds middlewares to all routes. Should be used when a middleware should be execute all all routes (e.g. CORS).
func (r *Router) Use(mid ...mux.MiddlewareFunc) {
	r.r.Use(mid...)
}

// Handler returns the configured router http handler.
func (r *Router) Handler() http.Handler {
	return r.r
}
