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
	uuid := "af227176-ed79-4670-93dd-c98ffa0f9f9e"
	path := fmt.Sprintf("/tables/%s", uuid)
	req, err := http.NewRequest("GET", path, nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/tables/{uuid}", systemController.GetTable)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	//nolint
	expJSON := `{
		"external_url":"https://tableland.com/tables/af227176-ed79-4670-93dd-c98ffa0f9f9e",
		"image":"https://hub.textile.io/thread/bafkqtqxkgt3moqxwa6rpvtuyigaoiavyewo67r3h7gsz4hov2kys7ha/buckets/bafzbeicpzsc423nuninuvrdsmrwurhv3g2xonnduq4gbhviyo5z4izwk5m/todo-list.png",
		"attributes":[{"display_type":"date","trait_type":"created","value":1546360800}]
	}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestInvalidUUID(t *testing.T) {
	uuid := "invalid uuid"
	path := fmt.Sprintf("/tables/%s", uuid)
	req, err := http.NewRequest("GET", path, nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/tables/{uuid}", systemController.GetTable)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusUnprocessableEntity, rr.Code)

	expJSON := `{"message": "Invalid uuid"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestTableNotFoundMock(t *testing.T) {
	uuid := "af227176-ed79-4670-93dd-c98ffa0f9f9e"
	path := fmt.Sprintf("/tables/%s", uuid)
	req, err := http.NewRequest("GET", path, nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockErrService()
	systemController := NewSystemController(systemService)

	router := mux.NewRouter()
	router.HandleFunc("/tables/{uuid}", systemController.GetTable)

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

	expJSON := `{"address": "some-address", "created_at": "0001-01-01T00:00:00Z"}`
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
