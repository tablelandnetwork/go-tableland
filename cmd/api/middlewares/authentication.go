package middlewares

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/textileio/go-tableland/pkg/errors"
	"github.com/textileio/go-tableland/pkg/jwt"
)

func Authentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(errors.ServiceError{Message: "no authorization header provided"})
			return
		}

		parts := strings.Split(authorization, "Bearer ")
		if len(parts) != 2 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(errors.ServiceError{Message: "malformed authorization header provided"})
			return
		}

		j, err := jwt.Parse(parts[1])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(errors.ServiceError{Message: fmt.Sprintf("parsing jwt: %v", err)})
			return
		}

		if err := j.Verify(); err != nil {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(errors.ServiceError{Message: fmt.Sprintf("validating jwt: %v", err)})
			return
		}

		next.ServeHTTP(w, r)
	})
}
