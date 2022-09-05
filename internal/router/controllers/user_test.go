package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

func TestUserController(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest("GET", "/chain/69/tables/100/id/1", nil)
	require.NoError(t, err)

	userController := NewUserController(&runnerMock{})

	router := mux.NewRouter()
	router.HandleFunc("/chain/{chainID}/tables/{id}/{key}/{value}", userController.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `{"columns":[{"name":"id"},{"name":"description"},{"name":"image"},{"name":"external_url"},{"name":"base"},{"name":"eyes"},{"name":"mouth"},{"name":"level"},{"name":"stamina"},{"name":"personality"},{"name":"aqua_power"},{"name":"stamina_increase"},{"name":"generation"}],"rows":[[1,"Friendly OpenSea Creature that enjoys long swims in the ocean.","https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png","https://example.com/?token_id=3","Starfish","Big","Surprised",5,1.4,"Sad",40,10,2]]}` // nolint
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestUserControllerERC721Metadata(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest("GET", "/chain/69/tables/100/id/1?format=erc721&name=id&image=image&description=description&external_url=external_url&attributes[0][column]=base&attributes[0][trait_type]=Base&attributes[1][column]=eyes&attributes[1][trait_type]=Eyes&attributes[2][column]=mouth&attributes[2][trait_type]=Mouth&attributes[3][column]=level&attributes[3][trait_type]=Level&attributes[4][column]=stamina&attributes[4][trait_type]=Stamina&attributes[5][column]=personality&attributes[5][trait_type]=Personality&attributes[6][column]=aqua_power&attributes[6][display_type]=boost_number&attributes[6][trait_type]=Aqua%20Power&attributes[7][column]=stamina_increase&attributes[7][display_type]=boost_percentage&attributes[7][trait_type]=Stamina%20Increase&attributes[8][column]=generation&attributes[8][display_type]=number&attributes[8][trait_type]=Generation", nil) // nolint
	require.NoError(t, err)

	userController := NewUserController(&runnerMock{})

	router := mux.NewRouter()
	router.HandleFunc("/chain/{chainID}/tables/{id}/{key}/{value}", userController.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `{"image":"https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png","external_url":"https://example.com/?token_id=3","description":"Friendly OpenSea Creature that enjoys long swims in the ocean.","name":"#1","attributes":[{"trait_type":"Base","value":"Starfish"},{"trait_type":"Eyes","value":"Big"},{"trait_type":"Mouth","value":"Surprised"},{"trait_type":"Level","value":5},{"trait_type":"Stamina","value":1.4},{"trait_type":"Personality","value":"Sad"},{"display_type":"boost_number","trait_type":"Aqua Power","value":40},{"display_type":"boost_percentage","trait_type":"Stamina Increase","value":10},{"display_type":"number","trait_type":"Generation","value":2}]}` // nolint
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestUserControllerInvalidColumn(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest("GET", "/chain/69/tables/100/invalid_column/0", nil)
	require.NoError(t, err)

	userController := NewUserController(&badRequestRunnerMock{})

	router := mux.NewRouter()
	router.HandleFunc("/chain/{chainID}/tables/{id}/{key}/{value}", userController.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	expJSON := `{"message": "Bad query result"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestUserControllerRowNotFound(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest("GET", "/chain/69/tables/100/id/1", nil)
	require.NoError(t, err)

	userController := NewUserController(&notFoundRunnerMock{})

	router := mux.NewRouter()
	router.HandleFunc("/chain/{chainID}/tables/{id}/{key}/{value}", userController.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)

	expJSON := `{"message": "Row not found"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestUserControllerTableQuery(t *testing.T) {
	userController := NewUserController(&queryRunnerMock{})

	router := mux.NewRouter()
	router.HandleFunc("/query", userController.GetTableQuery)

	// Columns mode
	req, err := http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B", nil)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp := `{"columns":[{"name":"id"},{"name":"eyes"},{"name":"mouth"}],"rows":[[1,"Big","Surprised"],[2,"Medium","Sad"],[3,"Small","Happy"]]}` // nolint
	require.JSONEq(t, exp, rr.Body.String())

	// Rows mode
	req, err = http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B&mode=rows", nil)
	require.NoError(t, err)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp = `[[1,"Big","Surprised"],[2,"Medium","Sad"],[3,"Small","Happy"]]`
	require.JSONEq(t, exp, rr.Body.String())

	// JSON mode
	req, err = http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B&mode=json", nil)
	require.NoError(t, err)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp = `[{"eyes":"Big","id":1,"mouth":"Surprised"},{"eyes":"Medium","id":2,"mouth":"Sad"},{"eyes":"Small","id":3,"mouth":"Happy"}]` // nolint
	require.JSONEq(t, exp, rr.Body.String())

	// CSV mode
	req, err = http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B&mode=csv", nil)
	require.NoError(t, err)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp = `id,eyes,mouth
1,"Big","Surprised"
2,"Medium","Sad"
3,"Small","Happy"
`
	require.Equal(t, exp, rr.Body.String())

	// List mode
	req, err = http.NewRequest("GET", "/query?s=select%20*%20from%20foo%3B&mode=list", nil)
	require.NoError(t, err)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp = `1|"Big"|"Surprised"
2|"Medium"|"Sad"
3|"Small"|"Happy"
`
	require.Equal(t, exp, rr.Body.String())
}

type runnerMock struct {
	counter int
}

func (rm *runnerMock) RunReadQuery(
	_ context.Context,
	_ string,
) (interface{}, error) {
	if rm.counter == 0 {
		rm.counter++
		return &sqlstore.UserRows{
			Rows: [][]*sqlstore.ColValue{{sqlstore.OtherUserValue("foo")}},
		}, nil
	}
	return &sqlstore.UserRows{
		Columns: []sqlstore.UserColumn{
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
		Rows: [][]*sqlstore.ColValue{
			{
				sqlstore.OtherUserValue(1),
				sqlstore.OtherUserValue("Friendly OpenSea Creature that enjoys long swims in the ocean."),
				sqlstore.OtherUserValue("https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png"),
				sqlstore.OtherUserValue("https://example.com/?token_id=3"),
				sqlstore.OtherUserValue("Starfish"),
				sqlstore.OtherUserValue("Big"),
				sqlstore.OtherUserValue("Surprised"),
				sqlstore.OtherUserValue(5),
				sqlstore.OtherUserValue(1.4),
				sqlstore.OtherUserValue("Sad"),
				sqlstore.OtherUserValue(40),
				sqlstore.OtherUserValue(10),
				sqlstore.OtherUserValue(2),
			},
		},
	}, nil
}

type badRequestRunnerMock struct{}

func (*badRequestRunnerMock) RunReadQuery(
	_ context.Context,
	_ string,
) (interface{}, error) {
	return "bad result", nil
}

type notFoundRunnerMock struct{}

func (*notFoundRunnerMock) RunReadQuery(
	_ context.Context,
	_ string,
) (interface{}, error) {
	return &sqlstore.UserRows{
		Columns: []sqlstore.UserColumn{
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
		Rows: [][]*sqlstore.ColValue{},
	}, nil
}

type queryRunnerMock struct{}

func (rm *queryRunnerMock) RunReadQuery(
	_ context.Context,
	_ string,
) (interface{}, error) {
	return &sqlstore.UserRows{
		Columns: []sqlstore.UserColumn{
			{Name: "id"},
			{Name: "eyes"},
			{Name: "mouth"},
		},
		Rows: [][]*sqlstore.ColValue{
			{
				sqlstore.OtherUserValue(1),
				sqlstore.OtherUserValue("Big"),
				sqlstore.OtherUserValue("Surprised"),
			},
			{
				sqlstore.OtherUserValue(2),
				sqlstore.OtherUserValue("Medium"),
				sqlstore.OtherUserValue("Sad"),
			},
			{
				sqlstore.OtherUserValue(3),
				sqlstore.OtherUserValue("Small"),
				sqlstore.OtherUserValue("Happy"),
			},
		},
	}, nil
}
