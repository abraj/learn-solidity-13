package geth

import (
	"bytes"
	"log"

	"github.com/ethereum/go-ethereum/rlp"
)

func RlpEncode(data interface{}) []byte {
	var buffer bytes.Buffer
	if err := rlp.Encode(&buffer, data); err != nil {
		log.Fatalf("Failed to RLP encode: %v", err)
	}
	return buffer.Bytes()
}

func RlpDecode[T any](dataBytes []byte) T {
	var data T
	var buffer bytes.Buffer
	buffer.Write(dataBytes)
	if err := rlp.Decode(&buffer, &data); err != nil {
		log.Fatalf("Failed to RLP decode: %v", err)
	}
	return data
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
// 	dataBytes := RlpEncode(person)
// 	fmt.Printf("Encoded RLP data: %x\n", dataBytes)

// 	// Decode the data back
// 	decodedPerson := RlpDecode[Person](dataBytes)
// 	fmt.Printf("Decoded data: %+v\n", decodedPerson)
// }
