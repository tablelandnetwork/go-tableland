package middlewares

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-limiter/memorystore"
)

// RateLimiterConfig specifies a default rate limiting configuration.
type RateLimiterConfig struct {
	Default RateLimiterRouteConfig
}

// RateLimiterRouteConfig specifies the maximum request per interval, and
// interval length for a rate limiting rule.
type RateLimiterRouteConfig struct {
	MaxRPI   uint64
	Interval time.Duration
}

// RateLimitController creates a new middleware to rate limit requests.
// It applies a priority based rate limiting key for the rate limiting:
// 1. If found, use an existing X-Forwarded-For IP included by a load-balancer in the infrastructure.
// 2. If 1. isn't present, it will use the connection remote address.
func RateLimitController(cfg RateLimiterConfig) (mux.MiddlewareFunc, error) {
	keyFunc := func(r *http.Request) (string, error) {
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

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(defaultRL.Handle(next).ServeHTTP)
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
