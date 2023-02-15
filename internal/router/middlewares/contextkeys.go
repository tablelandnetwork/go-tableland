package middlewares

// ContextKey is used to key context values.
type ContextKey int

// TODO: `ContextKeyAddress` is the wallet address, and since we are dropping the siwe token
//
//		we don't ever use, or save, this information. We should be able to remove this.
//	 does `ContextKeyChainID` come from the siwe token? If so, we should be able to remove this.
const (
	// ContextKeyAddress is used to store the address of the client for the incoming request.
	ContextKeyAddress ContextKey = iota
	// ContextKeyChainID is used to store the chain id of the client for the incoming request.
	ContextKeyChainID ContextKey = iota
	// ContextIPAddress is used to store the ip address of the client for the incoming request.
	ContextIPAddress ContextKey = iota
)
