package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p/core/crypto"
)

// Generate a new ECDSA private key
func generateECDSAKey() *ecdsa.PrivateKey {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return privKey
}

// Convert ECDSA private key to a libp2p compatible key
func ecdsaKeyToLibp2p(ecdsaKey *ecdsa.PrivateKey) crypto.PrivKey {
	libp2pPrivKey, _, err := crypto.ECDSAKeyPairFromKey(ecdsaKey)
	if err != nil {
		panic(err)
	}

	return libp2pPrivKey
}

// Convert ECDSA Private Key to Hex
func ecdsaPrivKeyToHex(priv *ecdsa.PrivateKey) string {
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		// panic(err)
		log.Fatalf("Failed to marshal private key: %v", err)
	}
	return hex.EncodeToString(privBytes)
}

// Convert ECDSA Hex back to ECDSA Private Key
func ecdsaHexToPrivKey(hexKey string) *ecdsa.PrivateKey {
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

// Generate a new ECDSA private key (as Hex)
func GenerateECDSAKeyHex() string {
	ecdsaPrivKey := generateECDSAKey()
	ecdsaPrivKeyStr := ecdsaPrivKeyToHex(ecdsaPrivKey)

	fmt.Println("ECDSA privateKey:", ecdsaPrivKeyStr)
	return ecdsaPrivKeyStr
}

// Decode ECDSA private key from raw hex bytes
func DecodeECDSAPrivateKey(ecdsaPrivKeyStr string) crypto.PrivKey {
	ecdsaPrivKey := ecdsaHexToPrivKey(ecdsaPrivKeyStr)
	privateKey := ecdsaKeyToLibp2p(ecdsaPrivKey)
	return privateKey
}

// Generate a new (Ed25519) private key (as Hex)
func GenerateEd25519KeyHex() string {
	privateKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		panic(err)
	}

	privBytes, err := privateKey.Raw()
	if err != nil {
		// panic(err)
		log.Fatalf("Failed to marshal private key: %v", err)
	}

	privateKeyStr := hex.EncodeToString(privBytes)
	fmt.Println("privateKey:", privateKeyStr)

	return privateKeyStr
}

// Decode (Ed25519) private key from raw hex bytes
func DecodeEd25519PrivateKey(privateKeyStr string) crypto.PrivKey {
	privBytes, err := hex.DecodeString(privateKeyStr)
	if err != nil {
		panic(err)
	}

	privateKey, err := crypto.UnmarshalEd25519PrivateKey(privBytes)
	if err != nil {
		panic(err)
	}

	// fmt.Println("privateKey:", privateKey)
	return privateKey
}
