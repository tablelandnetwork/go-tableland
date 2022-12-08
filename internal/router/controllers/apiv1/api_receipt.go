/*
 * Tableland Validator - OpenAPI 3.0
 *
 * In Tableland, Validators are the execution unit/actors of the protocol. They have the following responsibilities: - Listen to on-chain events to materialize Tableland-compliant SQL queries in a database engine (currently, SQLite by default). - Serve read-queries (e.g: SELECT * FROM foo_69_1) to the external world. - Serve state queries (e.g. list tables, get receipts, etc) to the external world.  In the 1.0.0 release of the Tableland Validator API, we've switched to a design first approach! You can now help us improve the API whether it's by making changes to the definition itself or to the code. That way, with time, we can improve the API in general, and expose some of the new features in OAS3.
 *
 * API version: 1.0.0
 * Contact: carson@textile.io
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package apiv1

import (
	"net/http"
)

func ReceiptByTransactionHash(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
}
