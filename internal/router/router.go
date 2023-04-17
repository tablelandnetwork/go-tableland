package router

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/textileio/go-tableland/internal/gateway"
	"github.com/textileio/go-tableland/internal/router/controllers"
	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
	"github.com/textileio/go-tableland/internal/router/middlewares"
	"github.com/textileio/go-tableland/internal/tableland"
)

// ConfiguredRouter returns a fully configured Router that can be used as an http handler.
func ConfiguredRouter(
	gateway gateway.Gateway,
	maxRPI uint64,
	rateLimInterval time.Duration,
	supportedChainIDs []tableland.ChainID,
) (*Router, error) {
	// General router configuration.
	router := newRouter()
	router.use(middlewares.CORS, middlewares.TraceID)

	cfg := middlewares.RateLimiterConfig{
		Default: middlewares.RateLimiterRouteConfig{
			MaxRPI:   maxRPI,
			Interval: rateLimInterval,
		},
	}
	rateLim, err := middlewares.RateLimitController(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating rate limit controller middleware: %s", err)
	}

	ctrl := controllers.NewController(gateway)

	// APIs V1
	if err := configureAPIV1Routes(router, supportedChainIDs, rateLim, ctrl); err != nil {
		return nil, fmt.Errorf("configuring API v1: %s", err)
	}

	return router, nil
}

func configureAPIV1Routes(
	router *Router,
	supportedChainIDs []tableland.ChainID,
	rateLim func(next http.Handler) http.Handler,
	userCtrl *controllers.Controller,
) error {
	handlers := map[string]struct {
		handler     http.HandlerFunc
		middlewares [](func(next http.Handler) http.Handler)
	}{
		"QueryByStatement": {
			userCtrl.GetTableQuery,
			[](func(next http.Handler) http.Handler){middlewares.WithLogging, rateLim},
		},
		"ReceiptByTransactionHash": {
			userCtrl.GetReceiptByTransactionHash,
			[](func(next http.Handler) http.Handler){middlewares.WithLogging, middlewares.RESTChainID(supportedChainIDs), rateLim},
		},
		"GetTableById": {
			userCtrl.GetTable,
			[](func(next http.Handler) http.Handler){middlewares.WithLogging, middlewares.RESTChainID(supportedChainIDs), rateLim},
		},
		"Version": {
			userCtrl.Version,
			[](func(next http.Handler) http.Handler){middlewares.WithLogging, rateLim},
		},
		"Health": {
			controllers.HealthHandler,
			[](func(next http.Handler) http.Handler){middlewares.WithLogging, rateLim},
		},
	}

	var specRoutesCount int
	if err := apiv1.NewRouter().Walk(func(route *chi.Route, _ *chi.Router, _ []*chi.Route) error {
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

// Router provides a nice api around chi.Router.
type Router struct {
	r *chi.Mux
}

// newRouter is a Mux HTTP router constructor.
func newRouter() *Router {
	r := chi.NewRouter()
	r.PathPrefix("/").Methods(http.MethodOptions) // accept OPTIONS on all routes and do nothing
	return &Router{r: r}
}

// get creates a subroute on the specified URI that only accepts GET. You can provide specific middlewares.
func (r *Router) get(uri string, f http.HandlerFunc, mid ...(func(next http.Handler) http.Handler)) {
	r.r.Route(uri, func (sr chi.Router) {
		sr.Get("/", f)
		sr.Use(mid...)
	})
}

// use adds middlewares to all routes. Should be used when a middleware should be execute all all routes (e.g. CORS).
func (r *Router) use(mid ...(func(next http.Handler) http.Handler)) {
	r.r.Use(mid...)
}

// Handler returns the configured router http handler.
func (r *Router) Handler() *chi.Mux {
	return r.r
}
