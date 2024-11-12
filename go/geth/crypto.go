package geth

import (
	"encoding/hex"

	"github.com/ethereum/go-ethereum/crypto"
)

// returns hash as hex encoded string
func Keccak256(data []byte) string {
	hexBytes := crypto.Keccak256(data)
	return hex.EncodeToString(hexBytes)
}
