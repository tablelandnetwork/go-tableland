package middlewares

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/textileio/go-tableland/pkg/errors"
)

type body struct {
	Params []struct {
		Controller string `json:"controller"`
	} `json:"params"`
}

// VerifyController makes sure the provided request controller matches the previously verified JWT.
func VerifyController(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")

		address := r.Context().Value(ContextKeyAddress)
		addressString, ok := address.(string)
		if !ok || addressString == "" {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "no address found in context"})
			return
		}

		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: fmt.Sprintf("error reading request body: %s", err)})
			return
		}

		r.Body = ioutil.NopCloser(bytes.NewBuffer(buf))

		var b body

		if err := json.Unmarshal(buf, &b); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: fmt.Sprintf("unable to decode body: %s", err)})
			return
		}

		if len(b.Params) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "no params found in body"})
			return
		}

		if address != b.Params[0].Controller {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errors.ServiceError{Message: "jwt address does not match controller address"})
			return
		}

		next.ServeHTTP(w, r)
	})
}
