package legacy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/mocks"
)

func TestRunReadQueryManyRows(t *testing.T) {
	tbl := mocks.NewTableland(t)
	tbl.EXPECT().RunReadQuery(mock.Anything, "SELECT * FROM bruno_69_7").Return(
		&tableland.TableData{
			Columns: []tableland.Column{{Name: "name"}, {Name: "age"}},
			Rows: [][]*tableland.ColumnValue{
				{tableland.OtherColValue("bob"), tableland.OtherColValue(40)},
				{tableland.OtherColValue("jane"), tableland.OtherColValue(30)},
			},
		},
		nil,
	)

	rpcService := NewRPCService(tbl)

	server := rpc.NewServer()
	err := server.RegisterName("tableland", rpcService)
	require.NoError(t, err)

	router := mux.NewRouter()
	router.Handle("/rpc", server)

	// Table output
	in := `{"jsonrpc":"2.0","method":"tableland_runReadQuery","id":1,"params":[{"statement":"SELECT * FROM bruno_69_7","output":"table"}]}` // nolint
	req, err := http.NewRequest("POST", "/rpc", strings.NewReader(in))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `{"jsonrpc":"2.0","id":1,"result":{"data":{"columns":[{"name":"name"},{"name":"age"}],"rows":[["bob",40],["jane",30]]}}}` // nolint
	require.JSONEq(t, expJSON, rr.Body.String())

	// Objects output
	in = `{"jsonrpc":"2.0","method":"tableland_runReadQuery","id":1,"params":[{"statement":"SELECT * FROM bruno_69_7","output":"objects"}]}` // nolint
	req, err = http.NewRequest("POST", "/rpc", strings.NewReader(in))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON = `{"jsonrpc":"2.0","id":1,"result":{"data":[{"age":40,"name":"bob"},{"age":30,"name":"jane"}]}}`
	require.JSONEq(t, expJSON, rr.Body.String())

	// Extract error
	in = `{"jsonrpc":"2.0","method":"tableland_runReadQuery","id":1,"params":[{"statement":"SELECT * FROM bruno_69_7","output":"objects","extract":true}]}` // nolint
	req, err = http.NewRequest("POST", "/rpc", strings.NewReader(in))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON = `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"formatting result: extracting values: can only extract values for result sets with one column but this has 2"}}` // nolint
	require.JSONEq(t, expJSON, rr.Body.String())

	// Unwrap error
	in = `{"jsonrpc":"2.0","method":"tableland_runReadQuery","id":1,"params":[{"statement":"SELECT * FROM bruno_69_7","output":"objects","unwrap":true}]}` // nolint
	req, err = http.NewRequest("POST", "/rpc", strings.NewReader(in))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON = `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"unwrapped results with more than one row aren't supported in JSON RPC API"}}` // nolint
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestRunReadQueryExtract(t *testing.T) {
	tbl := mocks.NewTableland(t)
	tbl.EXPECT().RunReadQuery(mock.Anything, "SELECT * FROM bruno_69_7").Return(
		&tableland.TableData{
			Columns: []tableland.Column{{Name: "name"}},
			Rows: [][]*tableland.ColumnValue{
				{tableland.OtherColValue("bob")},
				{tableland.OtherColValue("jane")},
			},
		},
		nil,
	)

	rpcService := NewRPCService(tbl)

	server := rpc.NewServer()
	err := server.RegisterName("tableland", rpcService)
	require.NoError(t, err)

	router := mux.NewRouter()
	router.Handle("/rpc", server)

	// Extract
	in := `{"jsonrpc":"2.0","method":"tableland_runReadQuery","id":1,"params":[{"statement":"SELECT * FROM bruno_69_7","output":"objects","extract":true}]}` // nolint
	req, err := http.NewRequest("POST", "/rpc", strings.NewReader(in))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `{"jsonrpc":"2.0","id":1,"result":{"data":["bob","jane"]}}` // nolint
	require.JSONEq(t, expJSON, rr.Body.String())
}

func TestRunReadQueryUnwrap(t *testing.T) {
	tbl := mocks.NewTableland(t)
	tbl.EXPECT().RunReadQuery(mock.Anything, "SELECT * FROM bruno_69_7").Return(
		&tableland.TableData{
			Columns: []tableland.Column{{Name: "name"}, {Name: "age"}},
			Rows: [][]*tableland.ColumnValue{
				{tableland.OtherColValue("bob"), tableland.OtherColValue(40)},
			},
		},
		nil,
	)

	rpcService := NewRPCService(tbl)

	server := rpc.NewServer()
	err := server.RegisterName("tableland", rpcService)
	require.NoError(t, err)

	router := mux.NewRouter()
	router.Handle("/rpc", server)

	// Unwrap
	in := `{"jsonrpc":"2.0","method":"tableland_runReadQuery","id":1,"params":[{"statement":"SELECT * FROM bruno_69_7","output":"objects","unwrap":true}]}` // nolint
	req, err := http.NewRequest("POST", "/rpc", strings.NewReader(in))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	expJSON := `{"jsonrpc":"2.0","id":1,"result":{"data":{"age":40,"name":"bob"}}}`
	require.JSONEq(t, expJSON, rr.Body.String())
}
