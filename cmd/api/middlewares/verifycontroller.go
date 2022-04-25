package middlewares

import (
	"encoding/json"
	"net/http"

	"github.com/textileio/go-tableland/pkg/errors"
)

// VerifyController makes sure the provided request controller matches the previously verified JWT.
func VerifyController(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")

		address := r.Context().Value(ContextKeyAddress)
		addressString, ok := address.(string)
		if !ok || addressString == "" {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "no address found in context"})
			return
		}

		next.ServeHTTP(w, r)
	})
}
