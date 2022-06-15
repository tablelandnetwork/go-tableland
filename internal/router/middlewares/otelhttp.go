package middlewares

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// OtelHTTP wraps the handler h with OTEL metrics.
func OtelHTTP(operation string) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return otelhttp.NewHandler(h, operation)
	}
}
