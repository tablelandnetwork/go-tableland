package middlewares

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sethvargo/go-limiter"
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
	MaxRPI    uint64
	Interval  time.Duration
	AllowList []string
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

func createRateLimiter(cfg RateLimiterRouteConfig, kf httplimit.KeyFunc) (*middleware, error) {
	defaultStore, err := memorystore.New(&memorystore.Config{
		Tokens:   cfg.MaxRPI,
		Interval: cfg.Interval,
	})
	if err != nil {
		return nil, fmt.Errorf("creating default memory: %s", err)
	}

	return &middleware{
		store:     defaultStore,
		keyFunc:   kf,
		allowlist: cfg.AllowList,
	}, nil
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

type middleware struct {
	store   limiter.Store
	keyFunc httplimit.KeyFunc

	// list of ip addresses not affected by rate limiter
	allowlist []string
}

// Handle returns the HTTP handler as a middleware. This handler calls Take() on
// the store and sets the common rate limiting headers. If the take is
// successful, the remaining middleware is called. If take is unsuccessful, the
// middleware chain is halted and the function renders a 429 to the caller with
// metadata about when it's safe to retry.
func (m *middleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Call the key function - if this fails, it's an internal server error.
		key, err := m.keyFunc(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// skip rate limiting checks if key is in allowlist
		for _, ip := range m.allowlist {
			if strings.EqualFold(key, ip) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Take from the store.
		limit, remaining, reset, ok, err := m.store.Take(ctx, key)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		resetTime := time.Unix(0, int64(reset)).UTC().Format(time.RFC1123)

		// Set headers (we do this regardless of whether the request is permitted).
		w.Header().Set("X-RateLimit-Limit", strconv.FormatUint(limit, 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatUint(remaining, 10))
		w.Header().Set("X-RateLimit-Reset", resetTime)

		// Fail if there were no tokens remaining.
		if !ok {
			w.Header().Set("Retry-After", resetTime)
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}

		// If we got this far, we're allowed to continue, so call the next middleware
		// in the stack to continue processing.
		next.ServeHTTP(w, r)
	})
}
