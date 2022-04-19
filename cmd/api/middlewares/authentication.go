package middlewares

import (
	"context"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/spruceid/siwe-go"
	"github.com/textileio/go-tableland/pkg/errors"
	"github.com/textileio/go-tableland/pkg/jwt"
)

var errSIWEWrongDomain = stderrors.New("SIWE domain isn't Tableland")

// Authentication is middleware that provides JWT authentication.
func Authentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "no authorization header provided"})
			return
		}

		parts := strings.Split(authorization, "Bearer ")
		if len(parts) != 2 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "malformed authorization header provided"})
			return
		}

		issuer, err := parseAuth(parts[1])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: fmt.Sprintf("parsing authorization: %v", err)})
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), ContextKeyAddress, issuer))

		next.ServeHTTP(w, r)
	})
}

func parseAuth(bearerToken string) (string, error) {
	j, err := jwt.Parse(bearerToken)
	// JWT
	if err == nil {
		if err := j.Verify(); err != nil {
			return "", fmt.Errorf("validating jwt: %v", err)
		}
		return j.Claims.Issuer, nil
	}

	// SIWE
	var siweAuthMsg struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}
	decodedSiwe, err := base64.StdEncoding.DecodeString(bearerToken)
	if err != nil {
		return "", fmt.Errorf("decoding base64 siwe authorization: %s", err)
	}
	if err := json.Unmarshal(decodedSiwe, &siweAuthMsg); err != nil {
		return "", fmt.Errorf("unmarshalling siwe auth message: %s", err)
	}
	msg, err := siwe.ParseMessage(siweAuthMsg.Message)
	if err != nil {
		return "", fmt.Errorf("parsing siwe: %s", err)
	}
	if msg.GetDomain() != "Tableland" {
		return "", errSIWEWrongDomain
	}
	if _, err := msg.Verify(siweAuthMsg.Signature, nil, nil); err != nil {
		return "", fmt.Errorf("checking siwe validity: %w", err)
	}
	return msg.GetAddress().String(), nil
}
