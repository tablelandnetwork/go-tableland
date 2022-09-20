package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/mocks"
)

func TestUserController(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest("GET", "/chain/69/tables/100/id/1", nil)
	require.NoError(t, err)

	userController := NewUserController(newTableRowRunnerMock(t))

	router := mux.NewRouter()
	router.HandleFunc("/chain/{chainID}/tables/{id}/{key}/{value}", userController.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `[{"id":1,"description":"Friendly OpenSea Creature that enjoys long swims in the ocean.","image":"https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png","external_url":"https://example.com/?token_id=3","base":"Starfish","eyes":"Big","mouth":"Surprised","level":5,"stamina":1.4,"personality":"Sad","aqua_power":40,"stamina_increase":10,"generation":2}]` // nolint
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestUserControllerERC721Metadata(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest("GET", "/chain/69/tables/100/id/1?format=erc721&name=id&image=image&description=description&external_url=external_url&attributes[0][column]=base&attributes[0][trait_type]=Base&attributes[1][column]=eyes&attributes[1][trait_type]=Eyes&attributes[2][column]=mouth&attributes[2][trait_type]=Mouth&attributes[3][column]=level&attributes[3][trait_type]=Level&attributes[4][column]=stamina&attributes[4][trait_type]=Stamina&attributes[5][column]=personality&attributes[5][trait_type]=Personality&attributes[6][column]=aqua_power&attributes[6][display_type]=boost_number&attributes[6][trait_type]=Aqua%20Power&attributes[7][column]=stamina_increase&attributes[7][display_type]=boost_percentage&attributes[7][trait_type]=Stamina%20Increase&attributes[8][column]=generation&attributes[8][display_type]=number&attributes[8][trait_type]=Generation", nil) // nolint
	require.NoError(t, err)

	userController := NewUserController(newTableRowRunnerMock(t))

	router := mux.NewRouter()
	router.HandleFunc("/chain/{chainID}/tables/{id}/{key}/{value}", userController.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `{"image":"https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png","external_url":"https://example.com/?token_id=3","description":"Friendly OpenSea Creature that enjoys long swims in the ocean.","name":"#1","attributes":[{"trait_type":"Base","value":"Starfish"},{"trait_type":"Eyes","value":"Big"},{"trait_type":"Mouth","value":"Surprised"},{"trait_type":"Level","value":5},{"trait_type":"Stamina","value":1.4},{"trait_type":"Personality","value":"Sad"},{"display_type":"boost_number","trait_type":"Aqua Power","value":40},{"display_type":"boost_percentage","trait_type":"Stamina Increase","value":10},{"display_type":"number","trait_type":"Generation","value":2}]}` // nolint
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestUserControllerBadQuery(t *testing.T) {
	t.Parallel()

	r := mocks.NewSQLRunner(t)
	r.EXPECT().RunReadQuery(mock.Anything, mock.Anything).Return(nil, errors.New("bad query error message"))

	req, err := http.NewRequest("GET", "/chain/69/tables/100/invalid_column/0", nil)
	require.NoError(t, err)

	userController := NewUserController(r)

	router := mux.NewRouter()
	router.HandleFunc("/chain/{chainID}/tables/{id}/{key}/{value}", userController.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	expJSON := `{"message": "bad query error message"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestUserControllerRowNotFound(t *testing.T) {
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

	userController := NewUserController(r)

	router := mux.NewRouter()
	router.HandleFunc("/chain/{chainID}/tables/{id}/{key}/{value}", userController.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)

	expJSON := `{"message": "Row not found"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestUserControllerQuery(t *testing.T) {
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

	userController := NewUserController(r)

	router := mux.NewRouter()
	router.HandleFunc("/query", userController.GetTableQuery)

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

	// Legacy 'mode' support

	// Mode = json
	req, err = http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B&mode=json", nil)
	require.NoError(t, err)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp = `[{"eyes":"Big","id":1,"mouth":"Surprised"},{"eyes":"Medium","id":2,"mouth":"Sad"},{"eyes":"Small","id":3,"mouth":"Happy"}]` // nolint
	require.JSONEq(t, exp, rr.Body.String())

	// Mode = list
	req, err = http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B&mode=list", nil)
	require.NoError(t, err)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp = "{\"eyes\":\"Big\",\"id\":1,\"mouth\":\"Surprised\"}\n{\"eyes\":\"Medium\",\"id\":2,\"mouth\":\"Sad\"}\n{\"eyes\":\"Small\",\"id\":3,\"mouth\":\"Happy\"}\n" // nolint
	wantStrings = parseJSONLString(exp)
	gotStrings = parseJSONLString(rr.Body.String())
	require.Equal(t, len(wantStrings), len(gotStrings))
	for i, wantString := range wantStrings {
		require.JSONEq(t, wantString, gotStrings[i])
	}
}

func TestUserControllerQueryExtracted(t *testing.T) {
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

	userController := NewUserController(r)

	router := mux.NewRouter()
	router.HandleFunc("/query", userController.GetTableQuery)

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
