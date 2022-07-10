package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

func TestUserController(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequest("GET", "/chain/69/tables/100/id/1?output=table", nil)
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

type runnerMock struct {
	counter int
}

func (rm *runnerMock) RunReadQuery(
	ctx context.Context,
	req tableland.RunReadQueryRequest,
) (tableland.RunReadQueryResponse, error) {
	if rm.counter == 0 {
		rm.counter++
		return tableland.RunReadQueryResponse{Result: []byte(`"foo"`)}, nil
	}
	return tableland.RunReadQueryResponse{
		Result: &sqlstore.UserRows{
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
			Rows: [][]interface{}{
				{
					1,
					"Friendly OpenSea Creature that enjoys long swims in the ocean.",
					"https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png",
					"https://example.com/?token_id=3",
					"Starfish",
					"Big",
					"Surprised",
					5,
					1.4,
					"Sad",
					40,
					10,
					2,
				},
			},
		},
	}, nil
}

type badRequestRunnerMock struct {
	counter int
}

func (b *badRequestRunnerMock) RunReadQuery(
	ctx context.Context,
	req tableland.RunReadQueryRequest,
) (tableland.RunReadQueryResponse, error) {
	if b.counter == 0 {
		b.counter++
		return tableland.RunReadQueryResponse{Result: []byte(`"foo"`)}, nil
	}
	return tableland.RunReadQueryResponse{}, errors.New("bad result")
}

type notFoundRunnerMock struct{}

func (*notFoundRunnerMock) RunReadQuery(
	ctx context.Context,
	req tableland.RunReadQueryRequest,
) (tableland.RunReadQueryResponse, error) {
	return tableland.RunReadQueryResponse{Result: []byte{}}, nil
}
