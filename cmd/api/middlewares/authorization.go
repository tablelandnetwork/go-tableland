package middlewares

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/context"
	"github.com/textileio/go-tableland/pkg/errors"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

// Authorization is middleware that checks the system auth store for an address.
func Authorization(store sqlstore.SQLStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-type", "application/json")

			address := context.Get(r, "address")
			addressString, ok := address.(string)
			if !ok || addressString == "" {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "no address in request context"})
				return
			}

			authorized, err := store.IsAuthorized(r.Context(), addressString)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: fmt.Sprintf("checking authorization status: %v", err)})
				return
			}

			if !authorized {
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: fmt.Sprintf("address %s not authorized", addressString)})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
