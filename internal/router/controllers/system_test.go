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

func TestGetTablesByMocked(t *testing.T) {
	t.Parallel()

	systemService := systemimpl.NewSystemMockService()
	ctrl := NewUserController(nil, systemService)

	t.Run("get table metadata", func(t *testing.T) {
		t.Parallel()
		req, err := http.NewRequest("GET", "/chain/1337/tables/100", nil)
		require.NoError(t, err)

		router := mux.NewRouter()
		router.HandleFunc("/chain/{chainID}/tables/{tableId}", ctrl.GetTable)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)

		//nolint
		expJSON := `{
			"name":"name-1",
			"external_url":"https://tableland.network/tables/100",
			"image":"https://bafkreifhuhrjhzbj4onqgbrmhpysk2mop2jimvdvfut6taiyzt2yqzt43a.ipfs.dweb.link",
			"attributes":[{"display_type":"date","trait_type":"created","value":1546360800}],
			"schema":{"columns":[{"name":"foo","type":"text"}]}
		}`
		require.JSONEq(t, expJSON, rr.Body.String())
	})

	t.Run("get tables by controller", func(t *testing.T) {
		t.Parallel()
		req, err := http.NewRequest("GET", "/chain/1337/tables/controller/0x2a891118Cf3a8FdeBb00109ea3ed4E33B82D960f", nil)
		require.NoError(t, err)

		router := mux.NewRouter()
		router.HandleFunc("/chain/{chainID}/tables/controller/{hash}", ctrl.GetTablesByController)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)

		//nolint
		expJSON := `[
			{
				"controller":"0x2a891118Cf3a8FdeBb00109ea3ed4E33B82D960f",
				"name":"test_1337_0",
				"structure":"0605f6c6705c7c1257edb2d61d94a03ad15f1d253a5a75525c6da8cda34a99ee"
			},
			{
				"controller":"0x2a891118Cf3a8FdeBb00109ea3ed4E33B82D960f",
				"name":"test2_1337_1",
				"structure":"0605f6c6705c7c1257edb2d61d94a03ad15f1d253a5a75525c6da8cda34a99ee"
			}]`
		require.JSONEq(t, expJSON, rr.Body.String())
	})

	t.Run("get tables by structure", func(t *testing.T) {
		t.Parallel()
		req, err := http.NewRequest("GET", "/chain/1337/tables/structure/0605f6c6705c7c1257edb2d61d94a03ad15f1d253a5a75525c6da8cda34a99eek", nil) // nolint
		require.NoError(t, err)

		router := mux.NewRouter()
		router.HandleFunc("/chain/{chainID}/tables/structure/{hash}", ctrl.GetTablesByStructureHash)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)

		//nolint
		expJSON := `[
			{
				"controller":"0x2a891118Cf3a8FdeBb00109ea3ed4E33B82D960f",
				"name":"test_1337_0",
				"structure":"0605f6c6705c7c1257edb2d61d94a03ad15f1d253a5a75525c6da8cda34a99ee"
			},
			{
				"controller":"0x2a891118Cf3a8FdeBb00109ea3ed4E33B82D960f",
				"name":"test2_1337_1",
				"structure":"0605f6c6705c7c1257edb2d61d94a03ad15f1d253a5a75525c6da8cda34a99ee"
			}]`
		require.JSONEq(t, expJSON, rr.Body.String())
	})

	t.Run("get schema by table name", func(t *testing.T) {
		t.Parallel()
		req, err := http.NewRequest("GET", "/schema/test_1337_0", nil) // nolint
		require.NoError(t, err)

		router := mux.NewRouter()
		router.HandleFunc("/schema/{table_name}", ctrl.GetSchemaByTableName)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)

		//nolint
		expJSON := `{
				"columns": [
					{
						"name" : "a",
						"type" : "int",
						"constraints" : ["PRIMARY KEY"]
					},
					{
						"name" : "b",
						"type" : "text",
						"constraints" : ["DEFAULT ''"]
					}				
				],
				"table_constraints": ["CHECK check (a > 0)"]
			}`
		require.JSONEq(t, expJSON, rr.Body.String())
	})
}

func TestGetTableWithInvalidID(t *testing.T) {
	t.Parallel()

	id := "invalid integer"
	path := fmt.Sprintf("/tables/%s", id)
	req, err := http.NewRequest("GET", path, nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockService()
	systemController := NewUserController(nil, systemService)

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
	systemController := NewUserController(nil, systemService)

	router := mux.NewRouter()
	router.HandleFunc("/tables/{tableId}", systemController.GetTable)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	expJSON := `{"message": "Failed to fetch metadata"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}
