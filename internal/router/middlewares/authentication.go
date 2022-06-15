package middlewares

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spruceid/siwe-go"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/errors"
)

var (
	errSIWEWrongDomain        = stderrors.New("SIWE domain isn't Tableland")
	siweDomain                = "Tableland"
	unauthenticatedRPCMethods = []string{
		"tableland_runReadQuery",
	}
)

// Authentication is middleware that provides JWT authentication.
func Authentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")

		fullBody, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "reading request body"})
			return
		}
		var rpcMethod struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(fullBody, &rpcMethod); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "request body doesn't have a method field"})
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(fullBody))
		if requiresAuthentication(rpcMethod.Method) {
			authorization := r.Header.Get("Authorization")
			if authorization == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "no authorization header provided"})
				return
			}

			parts := strings.Split(authorization, "Bearer ")
			if len(parts) != 2 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "malformed authorization header provided"})
				return
			}

			chainID, issuer, err := parseAuth(parts[1])
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: fmt.Sprintf("parsing authorization: %v", err)})
				return
			}

			r = r.WithContext(context.WithValue(r.Context(), ContextKeyAddress, strings.ToLower(issuer)))
			r = r.WithContext(context.WithValue(r.Context(), ContextKeyChainID, chainID))
		}

		next.ServeHTTP(w, r)
	})
}

func parseAuth(bearerToken string) (tableland.ChainID, string, error) {
	var siweAuthMsg struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}
	decodedSiwe, err := base64.StdEncoding.DecodeString(bearerToken)
	if err != nil {
		return 0, "", fmt.Errorf("decoding base64 siwe authorization: %s", err)
	}
	if err := json.Unmarshal(decodedSiwe, &siweAuthMsg); err != nil {
		return 0, "", fmt.Errorf("unmarshalling siwe auth message: %s", err)
	}
	msg, err := siwe.ParseMessage(siweAuthMsg.Message)
	if err != nil {
		return 0, "", fmt.Errorf("parsing siwe: %s", err)
	}
	if msg.GetDomain() != siweDomain {
		return 0, "", errSIWEWrongDomain
	}
	if _, err := msg.Verify(siweAuthMsg.Signature, &siweDomain, nil, nil); err != nil {
		return 0, "", fmt.Errorf("checking siwe validity: %w", err)
	}
	return tableland.ChainID(msg.GetChainID()), msg.GetAddress().String(), nil
}

func requiresAuthentication(rpcMethodName string) bool {
	for _, methodName := range unauthenticatedRPCMethods {
		if methodName == rpcMethodName {
			return false
		}
	}
	return true
}
