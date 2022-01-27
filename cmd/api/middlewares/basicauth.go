package middlewares

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

// BasicAuth is middleware that checks the expected username and password match the http basic auth values.
func BasicAuth(expectedUsername, expectedPassword string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()

			log.Info().Msg(fmt.Sprintf("expected user:pass - %s:%s", expectedUsername, expectedPassword))
			log.Info().Msg(fmt.Sprintf("received user:pass - %s:%s", username, password))

			if ok {
				usernameHash := sha256.Sum256([]byte(username))
				passwordHash := sha256.Sum256([]byte(password))
				expectedUsernameHash := sha256.Sum256([]byte(expectedUsername))
				expectedPasswordHash := sha256.Sum256([]byte(expectedPassword))

				log.Info().Msg(fmt.Sprintf("expected hashed user:pass - %s:%s", expectedUsernameHash, expectedPasswordHash))
				log.Info().Msg(fmt.Sprintf("received hashed user:pass - %s:%s", usernameHash, passwordHash))

				usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
				passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

				log.Info().Msg(fmt.Sprintf("username match: %v, password match: %v", usernameMatch, passwordMatch))

				if usernameMatch && passwordMatch {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
}
