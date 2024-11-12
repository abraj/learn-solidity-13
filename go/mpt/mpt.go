package mpt

import (
	"encoding/hex"
	"fmt"
	"libp2pdemo/geth"
	"libp2pdemo/utils"
	"log"
	"strconv"
)

// Merkle Patricia Trie (MPT) implementation

// TODO: Use compressed Patricia Radix Trie, rather than standard prefix trie
// Ref: https://chatgpt.com/c/6732a5e7-f130-8006-a295-9d46c56cc272

type MPT struct {
	root *MPTNode
}

type MPTNode struct {
	children []*MPTNode
	value    string
	hash     string
}

type Node struct {
	parent *MPTNode
	index  uint
}

func (t *MPT) init() *MPT {
	t.root = (&MPTNode{}).init()
	return t
}

func (n *MPTNode) init() *MPTNode {
	n.children = make([]*MPTNode, 16)
	n.updateHash()
	return n
}

func (n *MPTNode) updateHash() {
	n.hash = nodeHash(n)
}

func nodeHash(n *MPTNode) string {
	value := n.value
	for _, item := range n.children {
		if item != nil {
			value += item.hash
		}
	}
	return geth.Keccak256([]byte(value))
}

// NOTE: `key` is account address and must be hex string
func (t *MPT) Insert(key string, value interface{}) bool {
	if value == nil {
		log.Printf("Invalid value provided: %v\n", value)
		return false
	}

	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		fmt.Println("Error during insert operation! Unable to hex decode:", key, err)
		return false
	}

	path := geth.Keccak256(keyBytes)
	// fmt.Println(">> path:", path)

	stack := []Node{}
	node := t.root

	for _, nibble := range path {
		idx, _ := strconv.ParseInt(string(nibble), 16, 32)
		if node.children[idx] == nil {
			node.children[idx] = (&MPTNode{}).init()
		}
		stack = append(stack, Node{parent: node, index: uint(idx)})
		node = node.children[idx]
	}

	data, err := geth.RlpEncode(value)
	if err != nil {
		log.Printf("Error during RLP encode: %v\n", err)
		return false
	}

	node.value = data
	node.updateHash()

	for i := len(stack) - 1; i >= 0; i-- {
		parent := stack[i].parent
		parent.updateHash()
	}

	return true
}

// NOTE: `key` is account address and must be hex string
func (t *MPT) Update(key string, value interface{}) bool {
	return t.Insert(key, value)
}

// NOTE: `key` is account address and must be hex string
func (t *MPT) Get(key string, pdata interface{}) bool {
	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		fmt.Println("Error during get operation! Unable to hex decode:", key, err)
		return false
	}

	path := geth.Keccak256(keyBytes)
	// fmt.Println(">> path:", path)

	node := t.root

	for _, nibble := range path {
		idx, _ := strconv.ParseInt(string(nibble), 16, 32)
		if node.children[idx] == nil {
			return false // no item exists corresponding to the provided key
		}
		node = node.children[idx]
	}

	if node.value == "" {
		log.Printf("Invalid RLP encoded value stored: %v [%s]\n", node.value, key)
		return false
	}

	err = geth.RlpDecode(node.value, pdata)
	if err != nil {
		log.Printf("Error during RLP decode: %v\n", err)
		return false
	}
	return true
}

// NOTE: `key` is account address and must be hex string
func (t *MPT) Delete(key string) bool {
	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		fmt.Println("Error during delete operation! Unable to hex decode:", key, err)
		return false
	}

	path := geth.Keccak256(keyBytes)
	// fmt.Println(">> path:", path)

	stack := []Node{}
	node := t.root

	for _, nibble := range path {
		idx, _ := strconv.ParseInt(string(nibble), 16, 32)
		if node.children[idx] == nil {
			return false // no item exists corresponding to the provided key
		}
		stack = append(stack, Node{parent: node, index: uint(idx)})
		node = node.children[idx]
	}

	// remove the stored (RLP encoded) value
	node.value = ""
	node.updateHash()

	// prune unnecessary nodes, if any
	for i := len(stack) - 1; i >= 0; i-- {
		parent := stack[i].parent
		idx := stack[i].index
		child := parent.children[idx]

		// Check if child has any children or value. If not, prune it
		if utils.AllNil(child.children) && child.value == "" {
			parent.children[idx] = nil // prune
			parent.updateHash()
		} else {
			break // stop if we find a non-empty node
		}
	}

	return true
}

// // Example structure to encode
// type Person struct {
// 	Name string
// 	Age  uint
// }

// func Demo() {
// 	mpt := (&MPT{}).init()

// 	key := "2ED69CD751722FC552bc8C521846d55f6BD8F090"

// 	value := Person{Name: "Alice", Age: 30}
// 	success := mpt.Insert(key, value)
// 	fmt.Println("inserted:", success, mpt.root.hash)

// 	value = Person{Name: "Alice", Age: 33}
// 	success = mpt.Update(key, value)
// 	fmt.Println("updated:", success, mpt.root.hash)

// 	success = mpt.Delete(key)
// 	fmt.Println("deleted:", success, mpt.root.hash)

// 	var data Person
// 	found := mpt.Get(key, &data)
// 	if found {
// 		fmt.Printf("data: %+v\n", data)
// 	} else {
// 		fmt.Println("Not found!")
// 	}

// 	fmt.Println("rootHash:", mpt.root.hash)
// }
