package middlewares

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

// WithLogging logs requests and responses that contain useful information.
func WithLogging(h http.Handler) http.Handler {
	handler := func(rw http.ResponseWriter, req *http.Request) {
		loggedRW := &responseWriterLogger{
			ResponseWriter: rw,
		}
		h.ServeHTTP(loggedRW, req)

		if loggedRW.statusCode != http.StatusOK {
			log.Warn().Int("statusCode", loggedRW.statusCode).Msg("non-200 status code response")
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
