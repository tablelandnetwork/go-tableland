package middlewares

import (
	"context"
	"net/http"

	"github.com/rs/zerolog/log"
)

// WithLogging logs requests and responses that contain useful information.
func WithLogging(h http.Handler) http.Handler {
	handler := func(rw http.ResponseWriter, r *http.Request) {
		clientIP, err := extractClientIP(r)
		if err != nil {
			log.Warn().Err(err).Msg("can't extract client ip")
			clientIP = ""
		}

		r = r.WithContext(context.WithValue(r.Context(), ContextIPAddress, clientIP))

		loggedRW := &responseWriterLogger{
			ResponseWriter: rw,
		}
		h.ServeHTTP(loggedRW, r)

		if loggedRW.statusCode != http.StatusOK {
			log.Ctx(r.Context()).
				Warn().
				Int("statusCode", loggedRW.statusCode).
				Str("clientIP", clientIP).
				Msg("non-200 status code response")
		}
	}
	return http.HandlerFunc(handler)
}

type responseWriterLogger struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseWriterLogger) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.statusCode = statusCode
}
