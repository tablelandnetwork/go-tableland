package middlewares

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// TraceID creates a trace id for tracing. Every log goes with a trace id and it is also returned as a HTTP header.
func TraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uuid, err := uuid.NewRandom()
		if err != nil {
			log.Warn().Err(err).Msg("failed to generate a trace id")
			next.ServeHTTP(w, r)
			return
		}

		traceID := uuid.String()

		ctx := r.Context()
		logger := log.With().Str("traceId", traceID).Logger()
		r = r.WithContext(logger.WithContext(ctx))
		w.Header().Set("Trace-ID", traceID)

		next.ServeHTTP(w, r)
	})
}
