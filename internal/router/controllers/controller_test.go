package controllers

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/gateway"
	"github.com/textileio/go-tableland/internal/router/middlewares"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/mocks"
	"github.com/textileio/go-tableland/pkg/tables"
)

func TestQuery(t *testing.T) {
	r := mocks.NewGateway(t)
	r.EXPECT().RunReadQuery(mock.Anything, mock.AnythingOfType("string")).Return(
		&gateway.TableData{
			Columns: []gateway.Column{
				{Name: "id"},
				{Name: "eyes"},
				{Name: "mouth"},
			},
			Rows: [][]*gateway.ColumnValue{
				{
					gateway.OtherColValue(1),
					gateway.OtherColValue("Big"),
					gateway.OtherColValue("Surprised"),
				},
				{
					gateway.OtherColValue(2),
					gateway.OtherColValue("Medium"),
					gateway.OtherColValue("Sad"),
				},
				{
					gateway.OtherColValue(3),
					gateway.OtherColValue("Small"),
					gateway.OtherColValue("Happy"),
				},
			},
		},
		nil,
	)

	ctrl := NewController(r)

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
	r := mocks.NewGateway(t)
	r.EXPECT().RunReadQuery(mock.Anything, mock.AnythingOfType("string")).Return(
		&gateway.TableData{
			Columns: []gateway.Column{{Name: "name"}},
			Rows: [][]*gateway.ColumnValue{
				{gateway.OtherColValue("bob")},
				{gateway.OtherColValue("jane")},
				{gateway.OtherColValue("alex")},
			},
		},
		nil,
	)

	ctrl := NewController(r)

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

	g := mocks.NewGateway(t)
	g.EXPECT().GetTableMetadata(mock.Anything, mock.Anything, mock.Anything).Return(
		gateway.TableMetadata{
			Name:        "name-1",
			ExternalURL: "https://gateway.network/tables/100",
			Image:       "https://bafkreifhuhrjhzbj4onqgbrmhpysk2mop2jimvdvfut6taiyzt2yqzt43a.ipfs.dweb.link",
			Attributes: []gateway.TableMetadataAttribute{
				{
					DisplayType: "date",
					TraitType:   "created",
					Value:       1546360800,
				},
			},
			Schema: gateway.TableSchema{
				Columns: []gateway.ColumnSchema{
					{
						Name: "foo",
						Type: "text",
					},
				},
			},
		},
		nil,
	)

	ctrl := NewController(g)

	t.Run("get table metadata", func(t *testing.T) {
		t.Parallel()
		req, err := http.NewRequest("GET", "/api/v1/tables/1337/100", nil)
		require.NoError(t, err)

		req = req.WithContext(context.WithValue(req.Context(), middlewares.ContextKeyChainID, tableland.ChainID(1337)))

		router := mux.NewRouter()
		router.HandleFunc("/api/v1/tables/{chainID}/{tableId}", ctrl.GetTable)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)

		//nolint
		expJSON := `{
			"name":"name-1",
			"external_url":"https://gateway.network/tables/100",
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

	req = req.WithContext(context.WithValue(req.Context(), middlewares.ContextKeyChainID, tableland.ChainID(1337)))

	gateway := mocks.NewGateway(t)
	ctrl := NewController(gateway)

	router := mux.NewRouter()
	router.HandleFunc("/tables/{id}", ctrl.GetTable)

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

	req = req.WithContext(context.WithValue(req.Context(), middlewares.ContextKeyChainID, tableland.ChainID(1337)))

	g := mocks.NewGateway(t)
	g.EXPECT().GetTableMetadata(mock.Anything, mock.Anything, mock.Anything).Return(
		gateway.TableMetadata{},
		errors.New("failed"),
	)

	ctrl := NewController(g)

	router := mux.NewRouter()
	router.HandleFunc("/tables/{tableId}", ctrl.GetTable)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	expJSON := `{"message": "Failed to fetch metadata"}`
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestReceipt(t *testing.T) {
	r := mocks.NewGateway(t)
	r.EXPECT().GetReceiptByTransactionHash(mock.Anything, mock.Anything, mock.Anything).Return(
		gateway.Receipt{
			ChainID:       1337,
			BlockNumber:   1,
			IndexInBlock:  0,
			TxnHash:       "0xb5c8bd9430b6cc87a0e2fe110ece6bf527fa4f170a4bc8cd032f768fc5219838",
			TableIDs:      []tables.TableID{tables.TableID(*big.NewInt(1)), tables.TableID(*big.NewInt(2))},
			Error:         nil,
			ErrorEventIdx: nil,
		},
		true,
		nil,
	)

	ctrl := NewController(r)

	router := mux.NewRouter()
	router.HandleFunc("/receipt/{chainId}/{transactionHash}", ctrl.GetReceiptByTransactionHash)

	ctx := context.WithValue(context.Background(), middlewares.ContextKeyChainID, tableland.ChainID(1337))
	// Table format
	req, err := http.NewRequestWithContext(
		ctx, "GET", "/receipt/1337/0xb5c8bd9430b6cc87a0e2fe110ece6bf527fa4f170a4bc8cd032f768fc5219838", nil,
	)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	exp := `{"table_ids":["1","2"],"transaction_hash":"0xb5c8bd9430b6cc87a0e2fe110ece6bf527fa4f170a4bc8cd032f768fc5219838","block_number":1,"chain_id":1337}` // nolint
	require.JSONEq(t, exp, rr.Body.String())
}

func parseJSONLString(val string) []string {
	s := strings.TrimRight(val, "\n")
	return strings.Split(s, "\n")
}
