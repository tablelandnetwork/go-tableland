package middlewares

import (
	"fmt"
	"net/http"
)

func Authentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		fmt.Println(token)

		// adds token validation here

		next.ServeHTTP(w, r)
	})
}
