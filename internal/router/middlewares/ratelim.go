package middlewares

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-limiter/memorystore"
	"github.com/textileio/go-tableland/pkg/errors"
)

// RateLimiterConfig specifies a default rate limiting configuration, and optional custom rate limiting
// rules for a JSON RPC sub-route with path JSONRPCRoute. i.e: particular JSON RPC methods can have different
// rate limiting.
type RateLimiterConfig struct {
	Default RateLimiterRouteConfig

	JSONRPCRoute        string
	JSONRPCMethodLimits map[string]RateLimiterRouteConfig
}

// RateLimiterRouteConfig specifies the maximum request per interval, and
// interval length for a rate limiting rule.
type RateLimiterRouteConfig struct {
	MaxRPI   uint64
	Interval time.Duration
}

// RateLimitController creates a new middleware to rate limit requests.
// It applies a priority based rate limiting key for the rate limiting:
// 1. A "chain-address" was detected (i.e: via a signed SIWE).
// 2. If 1. isn't present, it will use an existing X-Forwarded-For IP included by a load-balancer in the infrastructure.
// 3. If 2. isn't present, it will use the connection remote address.
func RateLimitController(cfg RateLimiterConfig) (mux.MiddlewareFunc, error) {
	keyFunc := func(r *http.Request) (string, error) {
		// Use a chain address if present.
		address := r.Context().Value(ContextKeyAddress)
		ctrlAddress, ok := address.(string)
		if ok && ctrlAddress != "" {
			return ctrlAddress, nil
		}

		ip, err := extractClientIP(r)
		if err != nil {
			return "", fmt.Errorf("extract client ip: %s", err)
		}
		return ip, nil
	}

	defaultRL, err := createRateLimiter(cfg.Default, keyFunc)
	if err != nil {
		return nil, fmt.Errorf("creating default rate limiter: %s", err)
	}
	customRLs := make(map[string]*httplimit.Middleware, len(cfg.JSONRPCMethodLimits))
	for route, routeCfg := range cfg.JSONRPCMethodLimits {
		customRLs[route], err = createRateLimiter(routeCfg, keyFunc)
		if err != nil {
			return nil, fmt.Errorf("creating custom rate limiter for route %s: %s", route, err)
		}
	}

	return func(next http.Handler) http.Handler {
		defaultRLHandler := defaultRL.Handle(next)
		customRLHandlers := make(map[string]http.Handler, len(cfg.JSONRPCMethodLimits))
		for jsonMethod := range customRLs {
			customRLHandlers[jsonMethod] = customRLs[jsonMethod].Handle(next)
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// By default, set `m` with the default handler.
			m := defaultRLHandler

			// Now inspect if we should use some custom handler, if that's the case set `m` to that
			// value. If none is found, we'll use `m` as is (default).
			if r.URL.Path == cfg.JSONRPCRoute {
				fullBody, err := io.ReadAll(r.Body)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "reading request body"})
					return
				}
				var rpcMethod struct {
					Method string `json:"method"`
				}
				if err := json.Unmarshal(fullBody, &rpcMethod); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "request body doesn't have a method field"})
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(fullBody))
				if customLimiter, ok := customRLHandlers[rpcMethod.Method]; ok {
					m = customLimiter
				}
			}
			m.ServeHTTP(w, r)
		})
	}, nil
}

func createRateLimiter(cfg RateLimiterRouteConfig, kf httplimit.KeyFunc) (*httplimit.Middleware, error) {
	defaultStore, err := memorystore.New(&memorystore.Config{
		Tokens:   cfg.MaxRPI,
		Interval: cfg.Interval,
	})
	if err != nil {
		return nil, fmt.Errorf("creating default memory: %s", err)
	}
	m, err := httplimit.NewMiddleware(defaultStore, kf)
	if err != nil {
		return nil, fmt.Errorf("creating default httplimiter: %s", err)
	}
	return m, nil
}

func extractClientIP(r *http.Request) (string, error) {
	// Use X-Forwarded-For IP if present.
	// i.g: https://cloud.google.com/load-balancing/docs/https#x-forwarded-for_header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip := strings.Split(xff, ",")[0]
		return ip, nil
	}

	// Use the request remote address.
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", fmt.Errorf("getting ip from remote addr: %s", err)
	}
	return ip, nil
}
