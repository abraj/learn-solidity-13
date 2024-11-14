package vkt

import (
	"encoding/hex"
	"fmt"
	"libp2pdemo/geth"
	"libp2pdemo/utils"
	"math"
	"strconv"
	"strings"
)

// [Incomplete] Verkle tree implementation

// TODO: Have 256 branches per node, rather than 16 branches
// TODO: Use Vector commitment (Pedersen commitment) at each node, rather than SHA/Keccak hash
// Ref: https://eips.ethereum.org/EIPS/eip-6800
// Ref: https://chatgpt.com/c/67357fe7-a650-8006-9f51-88c536146755

type VKT struct {
	root *VKTNode
}

type VKTNode struct {
	children   []*VKTNode
	value      string
	commitment string
}

type Node struct {
	parent *VKTNode
	index  uint
}

func (t *VKT) init() *VKT {
	t.root = (&VKTNode{}).init()
	return t
}

func (n *VKTNode) init() *VKTNode {
	n.children = make([]*VKTNode, 16)
	n.updateCommitment()
	return n
}

func (n *VKTNode) updateCommitment() {
	n.commitment = nodeCommitment(n)
}

// TODO: Use Vector commitment (Pedersen commitment) at each node
func nodeCommitment(n *VKTNode) string {
	value := n.value
	for _, item := range n.children {
		if item != nil {
			value += item.commitment
		}
	}
	return geth.Keccak256([]byte(value))
}

// NOTE: `key` is account address and must be a 32 byte hex string
func (t *VKT) Insert(key string, value string) bool {
	_, err := hex.DecodeString(key)
	if err != nil {
		fmt.Println("Error during insert operation! Unable to hex decode key:", key, err)
		return false
	}
	if len(key) != 64 {
		fmt.Println("Invalid key length:", len(key), key)
		return false
	}

	path := key
	stack := []Node{}
	node := t.root

	for _, nibble := range path {
		idx, _ := strconv.ParseInt(string(nibble), 16, 32)
		if node.children[idx] == nil {
			node.children[idx] = (&VKTNode{}).init()
		}
		stack = append(stack, Node{parent: node, index: uint(idx)})
		node = node.children[idx]
	}

	node.value = value
	node.updateCommitment()

	for i := len(stack) - 1; i >= 0; i-- {
		parent := stack[i].parent
		parent.updateCommitment()
	}

	return true
}

// NOTE: `key` is account address and must be 32 byte hex string
func (t *VKT) Update(key string, value string) bool {
	return t.Insert(key, value)
}

// NOTE: `key` is account address and must be 32 byte hex string
func (t *VKT) Get(key string) string {
	_, err := hex.DecodeString(key)
	if err != nil {
		fmt.Println("Error during get operation! Unable to hex decode:", key, err)
		return ""
	}
	if len(key) != 64 {
		fmt.Println("Invalid key length:", len(key), key)
		return ""
	}

	path := key
	node := t.root

	for _, nibble := range path {
		idx, _ := strconv.ParseInt(string(nibble), 16, 32)
		if node.children[idx] == nil {
			return "" // no item exists corresponding to the provided key
		}
		node = node.children[idx]
	}

	return node.value
}

// NOTE: `key` is account address and must be hex string
func (t *VKT) Delete(key string) bool {
	_, err := hex.DecodeString(key)
	if err != nil {
		fmt.Println("Error during delete operation! Unable to hex decode:", key, err)
		return false
	}
	if len(key) != 64 {
		fmt.Println("Invalid key length:", len(key), key)
		return false
	}
	path := key

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
	node.updateCommitment()

	// prune unnecessary nodes, if any
	for i := len(stack) - 1; i >= 0; i-- {
		parent := stack[i].parent
		idx := stack[i].index
		child := parent.children[idx]

		// Check if child has any children or value. If not, prune it
		if utils.AllNil(child.children) && child.value == "" {
			parent.children[idx] = nil // prune
			parent.updateCommitment()
		} else {
			break // stop if we find a non-empty node
		}
	}

	return true
}

func GetNonceKey(addr string) string {
	addrBytes, err := hex.DecodeString(addr)
	if err != nil {
		fmt.Println("Error during field construction! Unable to hex decode:", addr, err)
		return ""
	}
	offset := "01" + strings.Repeat("0", 22)
	return hex.EncodeToString(addrBytes) + offset
}

func GetBalanceKey(addr string) string {
	addrBytes, err := hex.DecodeString(addr)
	if err != nil {
		fmt.Println("Error during field construction! Unable to hex decode:", addr, err)
		return ""
	}
	offset := "02" + strings.Repeat("0", 22)
	return hex.EncodeToString(addrBytes) + offset
}

func GetStorageKey(addr string, index int64) string {
	addrBytes, err := hex.DecodeString(addr)
	if err != nil {
		fmt.Println("Error during field construction! Unable to hex decode:", addr, err)
		return ""
	}
	indexStr := strconv.FormatInt(index, 10)
	hash := geth.Keccak256([]byte(indexStr))
	offset := "02" + hash[len(hash)-22:]
	return hex.EncodeToString(addrBytes) + offset
}

// int -> hex string (32 bytes)
func GetLeafValueInt(val int64) string {
	leafWidth := 32 // 32 bytes
	hexStr := strconv.FormatInt(val, 16)
	padding := strings.Repeat("0", 2*leafWidth-len(hexStr))
	value := padding + hexStr
	return value
}

// eth -> wei -> hex string (32 bytes)
func GetLeafValueEthWei(val float64) string {
	leafWidth := 32 // 32 bytes
	value := int64(val * math.Pow(10, 18))
	hexStr := strconv.FormatInt(value, 16)
	padding := strings.Repeat("0", 2*leafWidth-len(hexStr))
	valueStr := padding + hexStr
	return valueStr
}

// TODO: Implementing getLeafValueStr() for strings larger than 32 bytes might need additional data chunks (additional leaf nodes)
// func getLeafValueStr(val float64) string {
// }

// func Demo() {
// 	vkt := (&VKT{}).init()

// 	addr := "2ED69CD751722FC552bc8C521846d55f6BD8F090"

// 	key1 := GetNonceKey(addr)
// 	value1 := GetLeafValueInt(31) // nonce: 31
// 	success := vkt.Insert(key1, value1)
// 	fmt.Println("inserted:", success, vkt.root.commitment)

// 	value1 = GetLeafValueInt(32) // nonce: 32
// 	success = vkt.Update(key1, value1)
// 	fmt.Println("updated:", success, vkt.root.commitment)

// 	success = vkt.Delete(key1)
// 	fmt.Println("deleted:", success, vkt.root.commitment)

// 	value := vkt.Get(key1)
// 	if value != "" {
// 		fmt.Println("data:", value)
// 	} else {
// 		fmt.Println("Not found!")
// 	}

// 	key2 := GetBalanceKey(addr)
// 	value2 := GetLeafValueEthWei(2.8) // balance: 2.8 * 10^18 wei (2.8 eth)
// 	success = vkt.Insert(key2, value2)
// 	fmt.Println("inserted:", success, vkt.root.commitment)

// 	key3 := GetStorageKey(addr, 0)
// 	value3 := "hello"
// 	// TODO: Implementing getLeafValueStr() for strings larger than 32 bytes might need additional data chunks (additional leaf nodes)
// 	success = vkt.Insert(key3, value3)
// 	fmt.Println("inserted:", success, vkt.root.commitment)

// 	fmt.Println("rootCommitment:", vkt.root.commitment)
// }
