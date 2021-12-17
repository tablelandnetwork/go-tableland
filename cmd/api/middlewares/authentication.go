package middlewares

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/textileio/go-tableland/pkg/jwt"
)

func Authentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			http.Error(w, "no authorization header provided", http.StatusBadRequest)
			return
		}

		parts := strings.Split(authorization, "Bearer ")
		if len(parts) != 2 {
			http.Error(w, "malformed authorization header provided", http.StatusBadRequest)
			return
		}

		j, err := jwt.Parse(parts[1])
		if err != nil {
			http.Error(w, fmt.Sprintf("parsing jwt: %v", err), http.StatusBadRequest)
			return
		}

		if err := j.Verify(); err != nil {
			http.Error(w, fmt.Sprintf("validating jwt: %v", err), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
