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

func RESTChainID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		chainID, err := strconv.ParseInt(vars["chainID"], 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "no chain id in path"})
			return

		}
		r = r.WithContext(context.WithValue(r.Context(), ContextKeyChainID, tableland.ChainID(chainID)))

		next.ServeHTTP(w, r)
	})
}
