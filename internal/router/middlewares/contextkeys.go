package middlewares

// ContextKey is used to key context values.
type ContextKey int

const (
	// ContextKeyAddress is used to store the address of the client for the incoming request.
	ContextKeyAddress ContextKey = iota
	// ContextKeyChainID is used to store the chain id of the client for the incoming request.
	ContextKeyChainID ContextKey = iota
)
