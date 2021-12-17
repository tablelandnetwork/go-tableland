package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
	router.HandleFunc("/tables/{uuid}", systemController.GetTables)

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
	router.HandleFunc("/tables/{uuid}", systemController.GetTables)

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
	router.HandleFunc("/tables/{uuid}", systemController.GetTables)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	expJSON := `{"message": "Failed to fetch metadata"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}
