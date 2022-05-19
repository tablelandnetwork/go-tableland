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
	req, err := http.NewRequest("GET", "/tables/100/id/0", nil)
	require.NoError(t, err)

	userController := NewUserController(&runnerMock{})

	router := mux.NewRouter()
	router.HandleFunc("/tables/{id}/{key}/{value}", userController.GetTableRow)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	//nolint
	expJSON := `
{
  "name": "Dave Starbelly",
  "description": "Friendly OpenSea Creature that enjoys long swims in the ocean.",
  "image": "https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png",
  "external_url": "https://example.com/?token_id=3",
  "attributes": [
    {
      "trait_type": "base",
      "value": "narwhal"
    },
    {
      "trait_type": "eyes",
      "value": "sleepy"
    },
    {
      "trait_type": "mouth",
      "value": "cute"
    },
    {
      "trait_type": "level",
      "value": 4
    },
    {
      "trait_type": "stamina",
      "value": 90.2
    },
    {
      "trait_type": "personality",
      "value": "boring"
    },
    {
      "display_type": "boost_number",
      "trait_type": "aqua_power",
      "value": 10
    },
    {
      "display_type": "boost_percentage",
      "trait_type": "stamina_increase",
      "value": 5
    },
    {
      "display_type": "number",
      "trait_type": "generation",
      "value": 1
    }
  ]
}`
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
	req, err := http.NewRequest("GET", "/tables/100/id/0", nil)
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
				{Name: "name"},
				{Name: "description"},
				{Name: "image"},
				{Name: "external_url"},
				{Name: "attributes"},
			},
			Rows: [][]interface{}{
				{
					0,
					"Dave Starbelly",
					"Friendly OpenSea Creature that enjoys long swims in the ocean.",
					"https://storage.googleapis.com/opensea-prod.appspot.com/creature/3.png",
					"https://example.com/?token_id=3",
					[]map[string]interface{}{
						{
							"trait_type": "base",
							"value":      "narwhal",
						},
						{
							"trait_type": "eyes",
							"value":      "sleepy",
						},
						{
							"trait_type": "mouth",
							"value":      "cute",
						},
						{
							"trait_type": "level",
							"value":      4,
						},
						{
							"trait_type": "stamina",
							"value":      90.2,
						},
						{
							"trait_type": "personality",
							"value":      "boring",
						},
						{
							"display_type": "boost_number",
							"trait_type":   "aqua_power",
							"value":        10,
						},
						{
							"display_type": "boost_percentage",
							"trait_type":   "stamina_increase",
							"value":        5,
						},
						{
							"display_type": "number",
							"trait_type":   "generation",
							"value":        1,
						},
					},
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
				{Name: "name"},
				{Name: "description"},
				{Name: "image"},
				{Name: "external_url"},
				{Name: "attributes"},
			},
			Rows: [][]interface{}{},
		},
	}, nil
}
