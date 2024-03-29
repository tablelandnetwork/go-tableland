package middlewares

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/errors"
)

// RESTChainID adds to the request context the {chainID} that must be present in the REST path.
func RESTChainID(acceptedChainIDs []tableland.ChainID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)

			chainID, err := strconv.ParseInt(vars["chainId"], 10, 64)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "no chain id in path"})
				return
			}
			var found bool
			for _, acceptedChainID := range acceptedChainIDs {
				if chainID == int64(acceptedChainID) {
					found = true
					break
				}
			}
			if !found {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "unsupported chain id"})
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), ContextKeyChainID, tableland.ChainID(chainID)))
			next.ServeHTTP(w, r)
		})
	}
}
