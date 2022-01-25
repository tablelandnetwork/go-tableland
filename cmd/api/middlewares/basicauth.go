package middlewares

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
)

// BasicAuth is middleware that checks the expected username and password match the http basic auth values.
func BasicAuth(expectedUsername, expectedPassword string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if ok {
				usernameHash := sha256.Sum256([]byte(username))
				passwordHash := sha256.Sum256([]byte(password))
				expectedUsernameHash := sha256.Sum256([]byte(expectedUsername))
				expectedPasswordHash := sha256.Sum256([]byte(expectedPassword))

				usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
				passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

				if usernameMatch && passwordMatch {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
}
