package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	systemimpl "github.com/textileio/go-tableland/internal/system/impl"
)

func TestSystemControllerMock(t *testing.T) {
	req, err := http.NewRequest("GET", "/tables/100", nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/tables/{id}", systemController.GetTable)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	//nolint
	expJSON := `{
		"name":"name-1",
		"external_url":"https://tableland.network/tables/100",
		"image":"https://bafkreifhuhrjhzbj4onqgbrmhpysk2mop2jimvdvfut6taiyzt2yqzt43a.ipfs.dweb.link",
		"attributes":[{"display_type":"date","trait_type":"created","value":1546360800}]
	}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestInvalidID(t *testing.T) {
	id := "invalid integer"
	path := fmt.Sprintf("/tables/%s", id)
	req, err := http.NewRequest("GET", path, nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/tables/{id}", systemController.GetTable)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	expJSON := `{"message": "Invalid id format"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestTableNotFoundMock(t *testing.T) {
	req, err := http.NewRequest("GET", "/tables/100", nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockErrService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/tables/{id}", systemController.GetTable)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	expJSON := `{"message": "Failed to fetch metadata"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestAuthorize(t *testing.T) {
	req, err := http.NewRequest("POST", "/authorized-addresses", strings.NewReader("some-address"))
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/authorized-addresses", systemController.Authorize)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthorizeFail(t *testing.T) {
	req, err := http.NewRequest("POST", "/authorized-addresses", strings.NewReader("some-address"))
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockErrService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/authorized-addresses", systemController.Authorize)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	expJSON := `{"message": "Failed to authorize address"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestIsAuthorized(t *testing.T) {
	req, err := http.NewRequest("GET", "/authorized-addresses/some-address", nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/authorized-addresses/{address}", systemController.IsAuthorized)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `{"is_authorized": true}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestIsAuthorizedFail(t *testing.T) {
	req, err := http.NewRequest("GET", "/authorized-addresses/some-address", nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockErrService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/authorized-addresses/{address}", systemController.IsAuthorized)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	expJSON := `{"message": "Failed to check authorization"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestRevoke(t *testing.T) {
	req, err := http.NewRequest("DELETE", "/authorized-addresses/some-address", nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/authorized-addresses/{address}", systemController.Revoke)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestRevokeFail(t *testing.T) {
	req, err := http.NewRequest("DELETE", "/authorized-addresses/some-address", nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockErrService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/authorized-addresses/{address}", systemController.Revoke)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	expJSON := `{"message": "Failed to revoke address"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestGetAuthorizationRecord(t *testing.T) {
	req, err := http.NewRequest("GET", "/authorized-addresses/some-address/record", nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/authorized-addresses/{address}/record", systemController.GetAuthorizationRecord)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `{
		"address": "some-address",
		"created_at": "0001-01-01T00:00:00Z",
		"last_seen": null,
		"create_table_count": 0,
		"run_sql_count": 0
	}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestGetAuthorizationRecordFail(t *testing.T) {
	req, err := http.NewRequest("GET", "/authorized-addresses/some-address/record", nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockErrService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/authorized-addresses/{address}/record", systemController.GetAuthorizationRecord)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	expJSON := `{"message": "Failed to get authorization record"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestListAuthorized(t *testing.T) {
	req, err := http.NewRequest("GET", "/authorized-addresses", nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/authorized-addresses", systemController.ListAuthorized)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `[]`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestListAuthorizedFail(t *testing.T) {
	req, err := http.NewRequest("GET", "/authorized-addresses", nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockErrService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/authorized-addresses", systemController.ListAuthorized)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	expJSON := `{"message": "Failed to list authorization records"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}
