/*
 * Tableland Validator - OpenAPI 3.0
 *
 * In Tableland, Validators are the execution unit/actors of the protocol. They have the following responsibilities: - Listen to onchain events to materialize Tableland-compliant SQL queries in a database engine (currently, SQLite by default). - Serve read-queries (e.g., SELECT * FROM foo_69_1) to the external world. - Serve state queries (e.g., list tables, get receipts, etc) to the external world.  In the 1.0.0 release of the Tableland Validator API, we've switched to a design first approach! You can now help us improve the API whether it's by making changes to the definition itself or to the code. That way, with time, we can improve the API in general, and expose some of the new features in OAS3.  The API includes the following endpoints: - `/health`: Returns OK if the validator considers itself healthy. - `/version`: Returns version information about the validator daemon. - `/query`: Returns the results of a SQL read query against the Tableland network. - `/receipt/{chainId}/{transactionHash}`: Returns the status of a given transaction receipt by hash. - `/tables/{chainId}/{tableId}`: Returns information about a single table, including schema information.
 *
 * API version: 1.1.0
 * Contact: carson@textile.io
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package apiv1

type TransactionReceipt struct {
	// This field is deprecated
	TableId string `json:"table_id,omitempty"`

	TableIds []string `json:"table_ids,omitempty"`

	TransactionHash string `json:"transaction_hash,omitempty"`

	BlockNumber int64 `json:"block_number,omitempty"`

	ChainId int32 `json:"chain_id,omitempty"`

	Error_ string `json:"error,omitempty"`

	ErrorEventIdx int32 `json:"error_event_idx,omitempty"`
}
