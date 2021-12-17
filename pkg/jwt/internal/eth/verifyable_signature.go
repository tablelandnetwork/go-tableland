package eth

import (
	"bytes"
	"encoding/base64"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

//VerifyableSignature represents an Ethereum verifiable signature.
type VerifyableSignature struct {
	address       string
	signature     string
	signingTarget string
}

// NewVerifyableSignature creates a new VerifyableSignature.
func NewVerifyableSignature(address, signature, signingTarget string) *VerifyableSignature {
	return &VerifyableSignature{
		address:       address,
		signature:     signature,
		signingTarget: signingTarget,
	}
}

// Verify verifies the signature data.
func (v *VerifyableSignature) Verify() error {
	addressBytes := common.FromHex(v.address)
	address := common.BytesToAddress(addressBytes)

	signatureBytes, err := base64.RawURLEncoding.DecodeString(v.signature)
	if err != nil {
		return fmt.Errorf("decoding signature: %v", err)
	}

	// https://stackoverflow.com/questions/49085737/geth-ecrecover-invalid-signature-recovery-id
	// https://gist.github.com/dcb9/385631846097e1f59e3cba3b1d42f3ed#file-eth_sign_verify-go
	if signatureBytes[64] != 27 && signatureBytes[64] != 28 {
		return fmt.Errorf("sig[64] is not 27 or 28")
	}
	signatureBytes[64] -= 27

	sigPublicKeyECDSA, err := crypto.SigToPub(prefixedHash([]byte(v.signingTarget)), signatureBytes)
	if err != nil {
		return fmt.Errorf("extracting public key from signature: %v", err)
	}

	recoveredAddr := crypto.PubkeyToAddress(*sigPublicKeyECDSA)

	matches := bytes.Equal(address.Bytes(), recoveredAddr.Bytes())
	if !matches {
		return fmt.Errorf("recovered address %s doesn't equal provided address %s", recoveredAddr.String(), address.String())
	}

	signatureNoRecoverID := signatureBytes[:len(signatureBytes)-1] // remove recovery id
	verified := crypto.VerifySignature(
		crypto.FromECDSAPub(sigPublicKeyECDSA),
		prefixedHash([]byte(v.signingTarget)),
		signatureNoRecoverID,
	)
	if !verified {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

func prefixedHash(data []byte) []byte {
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(data), data)
	return crypto.Keccak256([]byte(msg))
}
