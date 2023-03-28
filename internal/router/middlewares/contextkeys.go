package middlewares

// ContextKey is used to key context values.
type ContextKey int

const (
	// ContextKeyChainID is used to store the chain id of the client for the incoming request,
	// this is found in the request path.
	ContextKeyChainID ContextKey = iota
	// ContextIPAddress is used to store the ip address of the client for the incoming request,
	// this is found in either the request IP or the x-forwarded header.
	ContextIPAddress ContextKey = iota
)
