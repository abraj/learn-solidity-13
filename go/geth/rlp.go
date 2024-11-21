package geth

import (
	"bytes"
	"encoding/hex"
	"log"

	"github.com/ethereum/go-ethereum/rlp"
)

// returns hex string for RLP encoded value
func RlpEncode(data interface{}) (string, error) {
	var buffer bytes.Buffer
	if err := rlp.Encode(&buffer, data); err != nil {
		log.Printf("Failed to RLP encode: %+v\n", data)
		return "", err
	}
	hexStr := hex.EncodeToString(buffer.Bytes())
	return hexStr, nil
}

// NOTE: encodedData must be a valid hex string (for RLP encoded value)
func RlpDecode(encodedData string, pdata interface{}) error {
	var buffer bytes.Buffer
	encodedBytes, err := hex.DecodeString(encodedData)
	if err != nil {
		log.Printf("encodedData is not a valid hex string: %s\n", encodedData)
		return err
	}
	buffer.Write(encodedBytes)
	if err := rlp.Decode(&buffer, pdata); err != nil {
		log.Printf("Failed to RLP decode: %s\n", encodedData)
		return err
	}
	return nil
}

// // Example structure to encode
// type Person struct {
// 	Name string
// 	Age  uint
// }

// func Demo() {
// 	// Initialize the data
// 	person := Person{Name: "Alice", Age: 30}

// 	// Encode the data
// 	data, _ := RlpEncode(person)
// 	fmt.Printf("Encoded RLP data: %s\n", data)

// 	// Decode the data back
// 	var decodedPerson Person
// 	RlpDecode(data, &decodedPerson)
// 	fmt.Printf("Decoded value: %+v\n", decodedPerson)
// }
