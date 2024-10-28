package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/sha3"
)

func RandomHex(bytesLength int) string {
	randBytes := make([]byte, bytesLength)
	_, err := rand.Read(randBytes)
	if err != nil {
		panic(err)
	}
	// fmt.Println("randBytes:", randBytes)

	randHex := hex.EncodeToString(randBytes)
	return randHex
}

func CreateSHA3Hash(dataStr string) string {
	data := []byte(dataStr)

	hash := sha3.New256()
	hash.Write(data)
	hashedBytes := hash.Sum(nil)

	hashValue := hex.EncodeToString(hashedBytes)
	return hashValue
}

// pad applies PKCS#7 padding to the input to make its length a multiple of the block size.
func pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// unpad removes PKCS#7 padding from the input.
func unpad(data []byte) ([]byte, error) {
	length := len(data)
	padding := int(data[length-1])
	if padding > length {
		return nil, fmt.Errorf("invalid padding")
	}
	return data[:length-padding], nil
}

// EncryptAES256CBC encrypts plaintext using AES-256 in CBC mode with the given key and IV.
func EncryptAES256CBC(keyStr, ivStr, plaintextStr string) (string, error) {
	key, err := hex.DecodeString(keyStr)
	if err != nil {
		return "", err
	}
	iv, err := hex.DecodeString(ivStr)
	if err != nil {
		return "", err
	}
	plaintext := []byte(plaintextStr)

	// Create a new AES cipher block with the provided key
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Pad the plaintext to a multiple of the block size
	paddedPlaintext := pad(plaintext, aes.BlockSize)

	// Encrypt the padded plaintext using CBC mode with the provided IV
	ciphertext := make([]byte, len(paddedPlaintext))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, paddedPlaintext)

	// Return the ciphertext as a hex-encoded string
	return hex.EncodeToString(ciphertext), nil
}

// DecryptAES256CBC decrypts a hex-encoded ciphertext using AES-256 in CBC mode with the given key and IV.
func DecryptAES256CBC(keyStr, ivStr, ciphertextHex string) (string, error) {
	key, err := hex.DecodeString(keyStr)
	if err != nil {
		return "", err
	}
	iv, err := hex.DecodeString(ivStr)
	if err != nil {
		return "", err
	}

	// Decode the hex string into a byte array
	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", err
	}

	// Create a new AES cipher block with the provided key
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Decrypt the ciphertext using CBC mode with the provided IV
	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	// Unpad the decrypted plaintext
	unpaddedPlaintext, err := unpad(plaintext)
	if err != nil {
		return "", err
	}

	return string(unpaddedPlaintext), nil
}
