package controllers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/router/middlewares"
	systemimpl "github.com/textileio/go-tableland/internal/system/impl"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/mocks"
)

func TestQuery(t *testing.T) {
	r := mocks.NewSQLRunner(t)
	r.EXPECT().RunReadQuery(mock.Anything, mock.AnythingOfType("string")).Return(
		&tableland.TableData{
			Columns: []tableland.Column{
				{Name: "id"},
				{Name: "eyes"},
				{Name: "mouth"},
			},
			Rows: [][]*tableland.ColumnValue{
				{
					tableland.OtherColValue(1),
					tableland.OtherColValue("Big"),
					tableland.OtherColValue("Surprised"),
				},
				{
					tableland.OtherColValue(2),
					tableland.OtherColValue("Medium"),
					tableland.OtherColValue("Sad"),
				},
				{
					tableland.OtherColValue(3),
					tableland.OtherColValue("Small"),
					tableland.OtherColValue("Happy"),
				},
			},
		},
		nil,
	)

	ctrl := NewController(r, nil)

	router := mux.NewRouter()
	router.HandleFunc("/query", ctrl.GetTableQuery)

	ctx := context.WithValue(context.Background(), middlewares.ContextIPAddress, strconv.Itoa(1))
	// Table format
	req, err := http.NewRequestWithContext(ctx, "GET", "/query?statement=select%20*%20from%20foo%3B&format=table", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp := `{"columns":[{"name":"id"},{"name":"eyes"},{"name":"mouth"}],"rows":[[1,"Big","Surprised"],[2,"Medium","Sad"],[3,"Small","Happy"]]}` // nolint
	require.JSONEq(t, exp, rr.Body.String())

	// Object format
	req, err = http.NewRequest("GET", "/query?statement=select%20*%20from%20foo%3B&format=objects", nil)
	require.NoError(t, err)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp = `[{"eyes":"Big","id":1,"mouth":"Surprised"},{"eyes":"Medium","id":2,"mouth":"Sad"},{"eyes":"Small","id":3,"mouth":"Happy"}]` // nolint
	require.JSONEq(t, exp, rr.Body.String())

	// Unwrapped object format
	req, err = http.NewRequest("GET", "/query?statement=select%20*%20from%20foo%3B&format=objects&unwrap=true", nil)
	require.NoError(t, err)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp = "{\"eyes\":\"Big\",\"id\":1,\"mouth\":\"Surprised\"}\n{\"eyes\":\"Medium\",\"id\":2,\"mouth\":\"Sad\"}\n{\"eyes\":\"Small\",\"id\":3,\"mouth\":\"Happy\"}\n" // nolint
	wantStrings := parseJSONLString(exp)
	gotStrings := parseJSONLString(rr.Body.String())
	require.Equal(t, len(wantStrings), len(gotStrings))
	for i, wantString := range wantStrings {
		require.JSONEq(t, wantString, gotStrings[i])
	}
}

func TestQueryExtracted(t *testing.T) {
	r := mocks.NewSQLRunner(t)
	r.EXPECT().RunReadQuery(mock.Anything, mock.AnythingOfType("string")).Return(
		&tableland.TableData{
			Columns: []tableland.Column{{Name: "name"}},
			Rows: [][]*tableland.ColumnValue{
				{tableland.OtherColValue("bob")},
				{tableland.OtherColValue("jane")},
				{tableland.OtherColValue("alex")},
			},
		},
		nil,
	)

	ctrl := NewController(r, nil)

	router := mux.NewRouter()
	router.HandleFunc("/query", ctrl.GetTableQuery)

	// Extracted object format
	req, err := http.NewRequest("GET", "/query?statement=select%20*%20from%20foo%3B&format=objects&extract=true", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp := `["bob","jane","alex"]`
	require.JSONEq(t, exp, rr.Body.String())

	// Extracted unwrapped object format
	req, err = http.NewRequest(
		"GET",
		"/query?statement=select%20*%20from%20foo%3B&format=objects&unwrap=true&extract=true",
		nil,
	)
	require.NoError(t, err)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp = "\"bob\"\n\"jane\"\n\"alex\"\n"
	wantStrings := parseJSONLString(exp)
	gotStrings := parseJSONLString(rr.Body.String())
	require.Equal(t, len(wantStrings), len(gotStrings))
	for i, wantString := range wantStrings {
		require.JSONEq(t, wantString, gotStrings[i])
	}
}

func TestGetTablesByMocked(t *testing.T) {
	t.Parallel()

	systemService := systemimpl.NewSystemMockService()
	ctrl := NewController(nil, systemService)

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
}

func TestGetTableWithInvalidID(t *testing.T) {
	t.Parallel()

	id := "invalid integer"
	path := fmt.Sprintf("/tables/%s", id)
	req, err := http.NewRequest("GET", path, nil)
	require.NoError(t, err)

	systemService := systemimpl.NewSystemMockService()
	systemController := NewController(nil, systemService)

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
	systemController := NewController(nil, systemService)

	router := mux.NewRouter()
	router.HandleFunc("/tables/{tableId}", systemController.GetTable)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	expJSON := `{"message": "Failed to fetch metadata"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func parseJSONLString(val string) []string {
	s := strings.TrimRight(val, "\n")
	return strings.Split(s, "\n")
}
