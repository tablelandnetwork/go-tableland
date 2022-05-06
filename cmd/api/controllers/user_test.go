package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

func TestUserController(t *testing.T) {
	req, err := http.NewRequest("GET", "/tables/100/id/1", nil)
	require.NoError(t, err)

	userController := NewUserController(&runnerMock{})

	router := mux.NewRouter()
	router.HandleFunc("/tables/{id}/{key}/{value}", userController.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `{"columns":[{"name":"id"},{"name":"description"},{"name":"image"},{"name":"external_url"},{"name":"att_base"},{"name":"att_eyes"},{"name":"att_mouth"},{"name":"att_level"},{"name":"att_stamina"},{"name":"att_personality"},{"name":"att_aqua_power"},{"name":"att_stamina_increase"},{"name":"att_generation"}],"rows":[[1,"Friendly OpenSea Creature that enjoys long swims in the ocean.","https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png","https://example.com/?token_id=3","narwhal","sleepy","cute",4,90.2,"boring",10,5,1]]}` // nolint
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestUserControllerERC721Metadata(t *testing.T) {
	req, err := http.NewRequest("GET", "/tables/100/id/1?format=erc721", nil)
	require.NoError(t, err)

	userController := NewUserController(&runnerMock{})

	router := mux.NewRouter()
	router.HandleFunc("/tables/{id}/{key}/{value}", userController.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `{"attributes":[{"trait_type":"base","value":"narwhal"},{"trait_type":"eyes","value":"sleepy"},{"trait_type":"mouth","value":"cute"},{"trait_type":"level","value":4},{"trait_type":"stamina","value":90.2},{"trait_type":"personality","value":"boring"},{"trait_type":"aqua_power","value":10},{"trait_type":"stamina_increase","value":5},{"trait_type":"generation","value":1}],"description":"Friendly OpenSea Creature that enjoys long swims in the ocean.","external_url":"https://example.com/?token_id=3","image":"https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png","name":"#1"}` // nolint
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestUserControllerInvalidColumn(t *testing.T) {
	req, err := http.NewRequest("GET", "/tables/100/invalid_column/0", nil)
	require.NoError(t, err)

	userController := NewUserController(&badRequestRunnerMock{})

	router := mux.NewRouter()
	router.HandleFunc("/tables/{id}/{key}/{value}", userController.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)

	expJSON := `{"message": "Bad query result"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestUserControllerRowNotFound(t *testing.T) {
	req, err := http.NewRequest("GET", "/tables/100/id/1", nil)
	require.NoError(t, err)

	userController := NewUserController(&notFoundRunnerMock{})

	router := mux.NewRouter()
	router.HandleFunc("/tables/{id}/{key}/{value}", userController.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)

	expJSON := `{"message": "Row not found"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

type runnerMock struct{}

func (*runnerMock) RunReadQuery(
	ctx context.Context,
	req tableland.RunReadQueryRequest) (tableland.RunReadQueryResponse, error) {
	return tableland.RunReadQueryResponse{
		Result: sqlstore.UserRows{
			Columns: []sqlstore.UserColumn{
				{Name: "id"},
				{Name: "description"},
				{Name: "image"},
				{Name: "external_url"},
				{Name: "att_base"},
				{Name: "att_eyes"},
				{Name: "att_mouth"},
				{Name: "att_level"},
				{Name: "att_stamina"},
				{Name: "att_personality"},
				{Name: "att_aqua_power"},
				{Name: "att_stamina_increase"},
				{Name: "att_generation"},
			},
			Rows: [][]interface{}{
				{
					1,
					"Friendly OpenSea Creature that enjoys long swims in the ocean.",
					"https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png",
					"https://example.com/?token_id=3",
					"narwhal",
					"sleepy",
					"cute",
					4,
					90.2,
					"boring",
					10,
					5,
					1,
				},
			},
		},
	}, nil
}

type badRequestRunnerMock struct{}

func (*badRequestRunnerMock) RunReadQuery(
	ctx context.Context,
	req tableland.RunReadQueryRequest) (tableland.RunReadQueryResponse, error) {
	return tableland.RunReadQueryResponse{
		Result: "bad result",
	}, nil
}

type notFoundRunnerMock struct{}

func (*notFoundRunnerMock) RunReadQuery(
	ctx context.Context,
	req tableland.RunReadQueryRequest) (tableland.RunReadQueryResponse, error) {
	return tableland.RunReadQueryResponse{
		Result: sqlstore.UserRows{
			Columns: []sqlstore.UserColumn{
				{Name: "id"},
				{Name: "description"},
				{Name: "image"},
				{Name: "external_url"},
				{Name: "att_base"},
				{Name: "att_eyes"},
				{Name: "att_mouth"},
				{Name: "att_level"},
				{Name: "att_stamina"},
				{Name: "att_personality"},
				{Name: "att_aqua_power"},
				{Name: "att_stamina_increase"},
				{Name: "att_generation"},
			},
			Rows: [][]interface{}{},
		},
	}, nil
}
