package wallet

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Wallet stores user's secret key and public key.
type Wallet struct {
	sk *ecdsa.PrivateKey
	pk *ecdsa.PublicKey
}

// NewWallet creates a new wallet.
func NewWallet(sk string) (*Wallet, error) {
	privateKey, err := crypto.HexToECDSA(sk)
	if err != nil {
		return &Wallet{}, fmt.Errorf("converting private key to ECDSA: %s", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return &Wallet{}, fmt.Errorf("casting public key to ECDSA: %s", err)
	}

	return &Wallet{
		sk: privateKey,
		pk: publicKeyECDSA,
	}, nil
}

// PrivateKey gets the private key.
func (w *Wallet) PrivateKey() *ecdsa.PrivateKey {
	return w.sk
}

// Address returns the hexadecimal wallet address.
func (w *Wallet) Address() common.Address {
	return common.HexToAddress(crypto.PubkeyToAddress(*w.pk).Hex())
}
