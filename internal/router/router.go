package router

import (
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/chains"
	"github.com/textileio/go-tableland/internal/router/controllers"
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
	maxRPI uint64,
	rateLimInterval time.Duration,
	parser parsing.SQLValidator,
	userStore *user.UserStore,
	chainStacks map[tableland.ChainID]chains.ChainStack,
) *Router {
	instrUserStore, err := sqlstoreimpl.NewInstrumentedUserStore(userStore)
	if err != nil {
		log.Fatal().Err(err).Msg("creating instrumented user store")
	}

	svc := getTablelandService(parser, instrUserStore, chainStacks)
	server := rpc.NewServer()
	if err := server.RegisterName("tableland", svc); err != nil {
		log.Fatal().Err(err).Msg("failed to register a json-rpc service")
	}
	userController := controllers.NewUserController(svc)

	stores := make(map[tableland.ChainID]sqlstore.SystemStore, len(chainStacks))
	for chainID, stack := range chainStacks {
		stores[chainID] = stack.Store
	}
	sysStore, err := systemimpl.NewSystemSQLStoreService(stores, extURLPrefix)
	if err != nil {
		log.Fatal().Err(err).Msg("creating system store")
	}
	systemService, err := systemimpl.NewInstrumentedSystemSQLStoreService(sysStore)
	if err != nil {
		log.Fatal().Err(err).Msg("instrumenting system sql store")
	}
	systemController := controllers.NewSystemController(systemService)

	// General router configuration.
	router := NewRouter()
	router.Use(middlewares.CORS, middlewares.TraceID)

	// RPC configuration.
	rateLim, err := middlewares.RateLimitController(maxRPI, rateLimInterval)
	if err != nil {
		log.Fatal().Err(err).Msg("creating rate limit controller middleware")
	}
	router.Post("/rpc", func(rw http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(rw, r)
	}, middlewares.Authentication, rateLim, middlewares.OtelHTTP("rpc"))

	// Gateway configuration.
	router.Get("/chain/{chainID}/tables/{id}", systemController.GetTable, middlewares.RESTChainID, middlewares.OtelHTTP("GetTable"))                                           // nolint
	router.Get("/chain/{chainID}/tables/{id}/{key}/{value}", userController.GetTableRow, middlewares.RESTChainID, middlewares.OtelHTTP("GetTableRow"))                         // nolint
	router.Get("/chain/{chainID}/tables/controller/{address}", systemController.GetTablesByController, middlewares.RESTChainID, middlewares.OtelHTTP("GetTablesByController")) // nolint
	router.Get("/query", userController.GetTableQuery, middlewares.OtelHTTP("GetTableQuery"))                                                                                  // nolint

	// Health endpoint configuration.
	router.Get("/healthz", healthHandler)
	router.Get("/health", healthHandler)

	return router
}

func getTablelandService(
	parser parsing.SQLValidator,
	userStore sqlstore.UserStore,
	chainStacks map[tableland.ChainID]chains.ChainStack) tableland.Tableland {
	instrumentedMesa, err := impl.NewInstrumentedTablelandMesa(
		impl.NewTablelandMesa(parser, userStore, chainStacks),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("instrumenting mesa")
	}
	return instrumentedMesa
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
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
