package demo

import (
	"libp2pdemo/geth"
	"libp2pdemo/utils"
	"reflect"
	// ssz "github.com/ferranbt/fastssz"
)

// TODO: Use SSZ encoding, rather than RLP encoding
func getMerkleRoot[T any](blockBody T) string {
	v := reflect.ValueOf(blockBody)
	// t := v.Type()

	container := []string{}
	for i := 0; i < v.NumField(); i++ {
		// field := t.Field(i)
		// fmt.Println(field.Name)
		value := v.Field(i)
		encodedValue, err := geth.RlpEncode(value.Interface())
		if err != nil {
			encodedValue = ""
		}
		container = append(container, encodedValue)
	}

	merkleHash := utils.MerkleHash(container)
	return "0x" + merkleHash
}
