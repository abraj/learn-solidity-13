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

func CreateSHA3Hash(dataStr string, saltStrHex string) string {
	hasher := sha3.New256()

	if saltStrHex != "" {
		saltBytes, err := hex.DecodeString(saltStrHex)
		if err != nil {
			fmt.Printf("Invalid salt (possibly not a hex string): %s %v\n", saltStrHex, err)
			return ""
		}
		hasher.Write(saltBytes)
	}

	dataBytes := []byte(dataStr)
	hasher.Write(dataBytes)

	hashedBytes := hasher.Sum(nil)

	hashValueHex := hex.EncodeToString(hashedBytes)
	return hashValueHex
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

func MerkleHash(list []string) string {
	if len(list) == 0 {
		return ""
	}

	curr_level := []string{}
	for _, item := range list {
		hash := CreateSHA3Hash(item, "")
		curr_level = append(curr_level, hash)
	}

	for len(curr_level) > 1 {
		next_level := []string{}
		for i := 0; i < len(curr_level); i += 2 {
			var data string
			if i+1 < len(curr_level) {
				data = curr_level[i] + curr_level[i+1]
			} else {
				data = curr_level[i] + curr_level[i] // duplicate last node
			}
			hash := CreateSHA3Hash(data, "")
			next_level = append(next_level, hash)
		}
		curr_level = next_level
	}

	return curr_level[0]
}

// func Test() {
// 	randHex := randomHex(32)
// 	fmt.Println("randHex:", randHex)

// 	hashValue := createSHA3Hash(randHex)
// 	fmt.Println("hash:", hashValue)

// 	key := randomHex(32)
// 	iv := randomHex(16)

// 	message := randHex
// 	ciphertext, err := encryptAES256CBC(key, iv, message)
// 	if err != nil {
// 		panic(err)
// 	}
// 	fmt.Println("ciphertext:", ciphertext)

// 	plaintext, err := decryptAES256CBC(key, iv, ciphertext)
// 	if err != nil {
// 		panic(err)
// 	}
// 	fmt.Println("randHex:", plaintext)
// }
