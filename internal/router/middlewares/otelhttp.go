package middlewares

import (
	"net/http"

	"github.com/textileio/go-tableland/pkg/metrics"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// OtelHTTP wraps the handler h with OTEL metrics.
func OtelHTTP(operation string) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return otelhttp.NewHandler(&labeledHandler{h: h}, operation)
	}
}

type labeledHandler struct {
	h http.Handler
}

func (lh *labeledHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	labeler, _ := otelhttp.LabelerFromContext(r.Context())
	labeler.Add(metrics.BaseAttrs...)
	lh.h.ServeHTTP(rw, r)
}
