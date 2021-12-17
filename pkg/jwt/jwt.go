package jwt

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt"
	"github.com/textileio/go-tableland/pkg/jwt/internal/eth"
)

type VerifyableSignature interface {
	Verify() error
}

type JWT struct {
	Type                string
	Algorhythm          string
	Network             string
	Blockchain          string
	Claims              jwt.StandardClaims
	verifyableSignature VerifyableSignature
}

func Parse(token string) (*JWT, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("token should have 3 parts but has %d", len(parts))
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decoding header: %v", err)
	}

	var headerJSON map[string]interface{}
	if err := json.Unmarshal(headerBytes, &headerJSON); err != nil {
		return nil, fmt.Errorf("unmarshaling header: %v", err)
	}

	alg, ok := headerJSON["alg"].(string)
	if !ok {
		return nil, fmt.Errorf("no alg present in header")
	}

	typ, ok := headerJSON["typ"].(string)
	if !ok {
		return nil, fmt.Errorf("no typ present in header")
	}

	kidString, ok := headerJSON["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("no kid present in header")
	}

	kidParts := strings.Split(kidString, ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("token should have 3 parts but has %d", len(parts))
	}

	// Claims

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decoding claims: %v", err)
	}

	var claims jwt.StandardClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, fmt.Errorf("unmarshaling claims: %v", err)
	}

	var vs VerifyableSignature
	switch alg {
	case "ETH":
		vs = eth.NewVerifyableSignature(claims.Issuer, parts[2], fmt.Sprintf("%s.%s", parts[0], parts[1]))
	default:
		return nil, fmt.Errorf("unsupported alg %s", alg)
	}

	return &JWT{
		Algorhythm:          alg,
		Type:                typ,
		Network:             kidParts[0],
		Blockchain:          kidParts[1],
		Claims:              claims,
		verifyableSignature: vs,
	}, nil
}

func (j *JWT) Verify() error {
	if err := j.Claims.Valid(); err != nil {
		return fmt.Errorf("validating claims: %v", err)
	}
	if err := j.verifyableSignature.Verify(); err != nil {
		return fmt.Errorf("verifying signature: %v", err)
	}
	return nil
}
