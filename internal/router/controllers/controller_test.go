package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	systemimpl "github.com/textileio/go-tableland/internal/system/impl"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/mocks"
)

func TestGetTableRow(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest("GET", "/chain/69/tables/100/id/1", nil)
	require.NoError(t, err)

	ctrl := NewController(newTableRowRunnerMock(t), nil)

	router := mux.NewRouter()
	router.HandleFunc("/chain/{chainID}/tables/{id}/{key}/{value}", ctrl.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `[{"id":1,"description":"Friendly OpenSea Creature that enjoys long swims in the ocean.","image":"https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png","external_url":"https://example.com/?token_id=3","base":"Starfish","eyes":"Big","mouth":"Surprised","level":5,"stamina":1.4,"personality":"Sad","aqua_power":40,"stamina_increase":10,"generation":2}]` // nolint
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestERC721Metadata(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest("GET", "/chain/69/tables/100/id/1?format=erc721&name=id&image=image&description=description&external_url=external_url&attributes[0][column]=base&attributes[0][trait_type]=Base&attributes[1][column]=eyes&attributes[1][trait_type]=Eyes&attributes[2][column]=mouth&attributes[2][trait_type]=Mouth&attributes[3][column]=level&attributes[3][trait_type]=Level&attributes[4][column]=stamina&attributes[4][trait_type]=Stamina&attributes[5][column]=personality&attributes[5][trait_type]=Personality&attributes[6][column]=aqua_power&attributes[6][display_type]=boost_number&attributes[6][trait_type]=Aqua%20Power&attributes[7][column]=stamina_increase&attributes[7][display_type]=boost_percentage&attributes[7][trait_type]=Stamina%20Increase&attributes[8][column]=generation&attributes[8][display_type]=number&attributes[8][trait_type]=Generation", nil) // nolint
	require.NoError(t, err)

	ctrl := NewController(newTableRowRunnerMock(t), nil)

	router := mux.NewRouter()
	router.HandleFunc("/chain/{chainID}/tables/{id}/{key}/{value}", ctrl.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `{"image":"https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png","external_url":"https://example.com/?token_id=3","description":"Friendly OpenSea Creature that enjoys long swims in the ocean.","name":"#1","attributes":[{"trait_type":"Base","value":"Starfish"},{"trait_type":"Eyes","value":"Big"},{"trait_type":"Mouth","value":"Surprised"},{"trait_type":"Level","value":5},{"trait_type":"Stamina","value":1.4},{"trait_type":"Personality","value":"Sad"},{"display_type":"boost_number","trait_type":"Aqua Power","value":40},{"display_type":"boost_percentage","trait_type":"Stamina Increase","value":10},{"display_type":"number","trait_type":"Generation","value":2}]}` // nolint
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestBadQuery(t *testing.T) {
	t.Parallel()

	r := mocks.NewSQLRunner(t)
	r.EXPECT().RunReadQuery(mock.Anything, mock.Anything).Return(nil, errors.New("bad query error message"))

	req, err := http.NewRequest("GET", "/chain/69/tables/100/invalid_column/0", nil)
	require.NoError(t, err)

	ctrl := NewController(r, nil)

	router := mux.NewRouter()
	router.HandleFunc("/chain/{chainID}/tables/{id}/{key}/{value}", ctrl.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	expJSON := `{"message": "bad query error message"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestRowNotFound(t *testing.T) {
	t.Parallel()

	r := mocks.NewSQLRunner(t)
	r.EXPECT().RunReadQuery(mock.Anything, mock.Anything).Return(
		&tableland.TableData{
			Columns: []tableland.Column{
				{Name: "id"},
				{Name: "description"},
				{Name: "image"},
				{Name: "external_url"},
				{Name: "base"},
				{Name: "eyes"},
				{Name: "mouth"},
				{Name: "level"},
				{Name: "stamina"},
				{Name: "personality"},
				{Name: "aqua_power"},
				{Name: "stamina_increase"},
				{Name: "generation"},
			},
			Rows: [][]*tableland.ColumnValue{},
		},
		nil,
	)

	req, err := http.NewRequest("GET", "/chain/69/tables/100/id/1", nil)
	require.NoError(t, err)

	ctrl := NewController(r, nil)

	router := mux.NewRouter()
	router.HandleFunc("/chain/{chainID}/tables/{id}/{key}/{value}", ctrl.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)

	expJSON := `{"message": "Row not found"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

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

	// Table output
	req, err := http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B&output=table", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp := `{"columns":[{"name":"id"},{"name":"eyes"},{"name":"mouth"}],"rows":[[1,"Big","Surprised"],[2,"Medium","Sad"],[3,"Small","Happy"]]}` // nolint
	require.JSONEq(t, exp, rr.Body.String())

	// Object output
	req, err = http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B&output=objects", nil)
	require.NoError(t, err)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp = `[{"eyes":"Big","id":1,"mouth":"Surprised"},{"eyes":"Medium","id":2,"mouth":"Sad"},{"eyes":"Small","id":3,"mouth":"Happy"}]` // nolint
	require.JSONEq(t, exp, rr.Body.String())

	// Unwrapped object output
	req, err = http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B&output=objects&unwrap=true", nil)
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

func TestLegacyQuery(t *testing.T) {
	r := mocks.NewSQLRunner(t)
	r.EXPECT().RunReadQuery(mock.Anything, mock.AnythingOfType("string")).Return(
		&tableland.TableData{
			Columns: []tableland.Column{
				{Name: "name"},
			},
			Rows: [][]*tableland.ColumnValue{
				{
					tableland.OtherColValue("Bob"),
				},
				{
					tableland.OtherColValue("John"),
				},
				{
					tableland.OtherColValue("Jane"),
				},
			},
		},
		nil,
	)

	ctrl := NewController(r, nil)

	router := mux.NewRouter()
	router.HandleFunc("/query", ctrl.GetTableQuery)

	// Mode = json
	req, err := http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B&mode=json", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp := `[{"name":"Bob"},{"name":"John"},{"name":"Jane"}]`
	require.JSONEq(t, exp, rr.Body.String())

	// Mode = list
	req, err = http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B&mode=list", nil)
	require.NoError(t, err)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp = "\"Bob\"\n\"John\"\n\"Jane\"\n"
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

	// Extracted object output
	req, err := http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B&output=objects&extract=true", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp := `["bob","jane","alex"]`
	require.JSONEq(t, exp, rr.Body.String())

	// Extracted unwrapped object output
	req, err = http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B&output=objects&unwrap=true&extract=true", nil)
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

func newTableRowRunnerMock(t *testing.T) SQLRunner {
	r := mocks.NewSQLRunner(t)
	r.EXPECT().RunReadQuery(mock.Anything, mock.Anything).Return(
		&tableland.TableData{
			Columns: []tableland.Column{{Name: "prefix"}},
			Rows:    [][]*tableland.ColumnValue{{tableland.OtherColValue("foo")}},
		},
		nil,
	).Once()
	r.EXPECT().RunReadQuery(mock.Anything, mock.Anything).Return(
		&tableland.TableData{
			Columns: []tableland.Column{
				{Name: "id"},
				{Name: "description"},
				{Name: "image"},
				{Name: "external_url"},
				{Name: "base"},
				{Name: "eyes"},
				{Name: "mouth"},
				{Name: "level"},
				{Name: "stamina"},
				{Name: "personality"},
				{Name: "aqua_power"},
				{Name: "stamina_increase"},
				{Name: "generation"},
			},
			Rows: [][]*tableland.ColumnValue{
				{
					tableland.OtherColValue(1),
					tableland.OtherColValue("Friendly OpenSea Creature that enjoys long swims in the ocean."),
					tableland.OtherColValue("https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png"),
					tableland.OtherColValue("https://example.com/?token_id=3"),
					tableland.OtherColValue("Starfish"),
					tableland.OtherColValue("Big"),
					tableland.OtherColValue("Surprised"),
					tableland.OtherColValue(5),
					tableland.OtherColValue(1.4),
					tableland.OtherColValue("Sad"),
					tableland.OtherColValue(40),
					tableland.OtherColValue(10),
					tableland.OtherColValue(2),
				},
			},
		},
		nil,
	).Once()
	return r
}

func parseJSONLString(val string) []string {
	s := strings.TrimRight(val, "\n")
	return strings.Split(s, "\n")
}
