package middlewares

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spruceid/siwe-go"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
)

func TestSIWE(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		siweToken := "eyJtZXNzYWdlIjoiVGFibGVsYW5kIHdhbnRzIHlvdSB0byBzaWduIGluIHdpdGggeW91ciBFdGhlcmV1bSBhY2NvdW50OlxuMHhkNTM1YkFkNTA0Q0RkNzdlMkM1MWRFMjZGNDE2NjkzREY3YTAxYWM4XG5cblNJV0UgTm90ZXBhZCBFeGFtcGxlXG5cblVSSTogaHR0cDovL2xvY2FsaG9zdDo0MzYxXG5WZXJzaW9uOiAxXG5DaGFpbiBJRDogNFxuTm9uY2U6IEhHVkJWMFdvYlFHb1ZWUUlzXG5Jc3N1ZWQgQXQ6IDIwMjItMDQtMTlUMTg6NDA6MDQuMDQ2WlxuRXhwaXJhdGlvbiBUaW1lOiAyMDUyLTA0LTE4VDE1OjA4OjE0LjgwNVoiLCJzaWduYXR1cmUiOiIweDk3NTFjNDI2MjNiYTZhNjc1OTA5YjEzMzVjZGI2NDc0ODU4MmY5OTMyMTQxOTBmZmM2MGE0OGRhN2UzOTNhMjcwMDkzMDgzZmRkMzI4ZTNkZjA2ODc3ZTY3MjQ2MWJhMjcwYmI2YjFiYmQxMGJmNTBiMTliMTg5MmExNDhiNzkzMWMifQ==" //nolint
		chainID, issuer, err := parseAuth(siweToken)
		require.NoError(t, err)
		require.Equal(t, "0xd535bAd504CDd77e2C51dE26F416693DF7a01ac8", issuer)
		require.Equal(t, tableland.ChainID(4), chainID)
	})
	t.Run("wrong domain", func(t *testing.T) {
		t.Parallel()

		siweToken := "eyJtZXNzYWdlIjoibG9jYWxob3N0OjQzNjEgd2FudHMgeW91IHRvIHNpZ24gaW4gd2l0aCB5b3VyIEV0aGVyZXVtIGFjY291bnQ6XG4weGQ1MzViQWQ1MDRDRGQ3N2UyQzUxZEUyNkY0MTY2OTNERjdhMDFhYzhcblxuU0lXRSBOb3RlcGFkIEV4YW1wbGVcblxuVVJJOiBodHRwOi8vbG9jYWxob3N0OjQzNjFcblZlcnNpb246IDFcbkNoYWluIElEOiA0XG5Ob25jZTogdHhEY1pOOUJ1NkhHbXpDdmRcbklzc3VlZCBBdDogMjAyMi0wNC0xOFQyMjoyNDoxNS4xNDRaXG5FeHBpcmF0aW9uIFRpbWU6IDIwNTItMDQtMThUMTU6MDg6MTQuODA1WiIsInNpZ25hdHVyZSI6IjB4MThiOTlmOTY3YjUzNjgxZWZiNTU0Mjk4ZmNkYjJmYjE5N2JiYjEwODU0MmM4Mzc3ZDM0MGE5Zjk0M2RkZTY4NzcwNWUyOTQ3OGZjNTI1MzYyZmU5OGU1ZWI2NzAxOTU3OWM3MzQ4ZThkMTVmNzhjOTRiZDdiNWIzMjdlOTQ3MTAxYyJ9" //nolint
		_, _, err := parseAuth(siweToken)
		require.ErrorIs(t, err, errSIWEWrongDomain)
	})
	t.Run("expired", func(t *testing.T) {
		t.Parallel()

		siweToken := "eyJtZXNzYWdlIjoiVGFibGVsYW5kIHdhbnRzIHlvdSB0byBzaWduIGluIHdpdGggeW91ciBFdGhlcmV1bSBhY2NvdW50OlxuMHhkNTM1YkFkNTA0Q0RkNzdlMkM1MWRFMjZGNDE2NjkzREY3YTAxYWM4XG5cblNJV0UgTm90ZXBhZCBFeGFtcGxlXG5cblVSSTogaHR0cDovL2xvY2FsaG9zdDo0MzYxXG5WZXJzaW9uOiAxXG5DaGFpbiBJRDogNFxuTm9uY2U6IDBPT3dzOERXSlE5OEJ2ZGZWXG5Jc3N1ZWQgQXQ6IDIwMjItMDQtMTlUMTg6NDc6NTMuMTUxWlxuRXhwaXJhdGlvbiBUaW1lOiAyMDIyLTA0LTE4VDE1OjA4OjE0LjgwNVoiLCJzaWduYXR1cmUiOiIweGViMjM4MGNiMjA0NmQzNzZiZWI3NjQ0YjBkYTE4ZTA4NWM4NmVlNTZhZGY1MjUzYTcwZDZiZGY2N2Q0MGRjMDAwMzk0ZDk3ZWQzOTA2YmI5ZDNkMTM0MWFmODg3YWFhYzE5YWNmY2QwNmE3ZTI0ODBlMGI0MDJhMzRhOTdkZjEzMWMifQ==" //nolint
		_, _, err := parseAuth(siweToken)
		var expErr *siwe.ExpiredMessage
		require.ErrorAs(t, err, &expErr)
	})
}

func TestOptionality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rpcMethodName string
		expStatusCode int
	}{
		{rpcMethodName: "tableland_runReadQuery", expStatusCode: http.StatusOK},
		{rpcMethodName: "tableland_relayWriteQuery", expStatusCode: http.StatusUnauthorized},
		{rpcMethodName: "tableland_validateCreateTable", expStatusCode: http.StatusUnauthorized},
		{rpcMethodName: "tableland_validateWriteQuery", expStatusCode: http.StatusUnauthorized},
		{rpcMethodName: "tableland_getReceipt", expStatusCode: http.StatusUnauthorized},
		{rpcMethodName: "tableland_setController", expStatusCode: http.StatusUnauthorized},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.rpcMethodName, func(t *testing.T) {
			t.Parallel()
			called := false
			next := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				called = true
			})

			body := bytes.NewReader([]byte(fmt.Sprintf(`{"method": "%s"}`, tc.rpcMethodName)))
			r := httptest.NewRequest("POST", "/rpc", body)
			rw := httptest.NewRecorder()

			h := Authentication(next)
			h.ServeHTTP(rw, r)

			require.Equal(t, tc.expStatusCode, rw.Code)
			require.Equal(t, requiresAuthentication(tc.rpcMethodName), !called)
		})
	}
}
