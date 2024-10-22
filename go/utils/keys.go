package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"log"

	"github.com/libp2p/go-libp2p/core/crypto"
)

// Convert ECDSA Private Key to Hex
func PrivKeyToHex(priv *ecdsa.PrivateKey) string {
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		// panic(err)
		log.Fatalf("Failed to marshal private key: %v", err)
	}
	return hex.EncodeToString(privBytes)
}

// Convert ECDSA Hex back to ECDSA Private Key
func HexToPrivKey(hexKey string) *ecdsa.PrivateKey {
	privBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		panic(err)
	}

	priv, err := x509.ParseECPrivateKey(privBytes)
	if err != nil {
		panic(err)
	}
	return priv

	// privKey.D.Cmp(restoredPrivKey.D) == 0
}

// Generate a new ECDSA private key
func GenerateECDSAKey() *ecdsa.PrivateKey {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return privKey
}

// Convert the ECDSA private key to a libp2p compatible key
func ConvertECDSAKeyToLibp2p(ecdsaKey *ecdsa.PrivateKey) crypto.PrivKey {
	libp2pPrivKey, _, err := crypto.ECDSAKeyPairFromKey(ecdsaKey)
	if err != nil {
		panic(err)
	}
	return libp2pPrivKey
}

// Generate a new ECDSA private key (as Hex)
func GenerateECDSAKeyHex() string {
	privKey1 := GenerateECDSAKey()
	privateKeyHex := PrivKeyToHex(privKey1)

	// fmt.Println("privateKey:", privateKeyHex)
	return privateKeyHex
}
