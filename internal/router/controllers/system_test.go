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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
