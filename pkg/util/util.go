package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spruceid/siwe-go"

	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/wallet"
)

// AuthorizationSIWEValue returns the "Bearer ..." string for the provided chainid and wallet.
func AuthorizationSIWEValue(chainID tableland.ChainID, wallet *wallet.Wallet, validFor time.Duration) (string, error) {
	opts := map[string]interface{}{
		"chainId":        int(chainID),
		"expirationTime": time.Now().Add(validFor),
		"nonce":          siwe.GenerateNonce(),
	}

	msg, err := siwe.InitMessage("Tableland", wallet.Address().Hex(), "https://staging.tableland.io", "1", opts)
	if err != nil {
		return "", fmt.Errorf("initializing siwe message: %v", err)
	}

	payload := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(msg.String()), msg.String())
	hash := crypto.Keccak256Hash([]byte(payload))
	signature, err := crypto.Sign(hash.Bytes(), wallet.PrivateKey())
	if err != nil {
		return "", fmt.Errorf("signing siwe message: %v", err)
	}
	signature[64] += 27

	bearerValue := struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}{Message: msg.String(), Signature: hexutil.Encode(signature)}
	bearer, err := json.Marshal(bearerValue)
	if err != nil {
		return "", fmt.Errorf("json marshaling signed siwe: %v", err)
	}

	return fmt.Sprintf("Bearer %s", base64.StdEncoding.EncodeToString(bearer)), nil
}
