package middlewares

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-limiter/memorystore"
)

func RateLimitController(maxRPI uint64, interval time.Duration, next http.Handler) (http.Handler, error) {
	controllerAsKey := func(r *http.Request) (string, error) {
		address := r.Context().Value(ContextKeyAddress)
		ctrlAddress, ok := address.(string)
		if !ok || ctrlAddress == "" {
			return "", errors.New("no controller address found in context")
		}
		return ctrlAddress, nil
	}

	store, err := memorystore.New(&memorystore.Config{
		Tokens:   maxRPI,
		Interval: interval,
	})
	m, err := httplimit.NewMiddleware(store, controllerAsKey)
	if err != nil {
		return nil, fmt.Errorf("creating httplimiter: %s", err)
	}
	return m.Handle(next), nil
}
