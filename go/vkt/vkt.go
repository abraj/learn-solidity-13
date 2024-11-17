package vkt

import (
	"context"
	"encoding/hex"
	"fmt"
	"libp2pdemo/geth"
	"libp2pdemo/utils"
	"log"
	"math"
	"strconv"
	"strings"

	ds "github.com/ipfs/go-datastore"
)

const vktPrefix = "/vkt/node"

// [Incomplete] Verkle tree implementation

// TODO: Have 256 branches per node, rather than 16 branches
// TODO: Use Vector commitment (Pedersen commitment) at each node, rather than SHA/Keccak hash
// Ref: https://eips.ethereum.org/EIPS/eip-6800
// Ref: https://chatgpt.com/c/67357fe7-a650-8006-9f51-88c536146755

type VKT struct {
	Root *VKTNode
}

type VKTNode struct {
	Commitment string
	value      string
	children   []*VKTNode
	stub       []string
}

// NOTE: all values exported for serialization
type VKTNodeObj struct {
	Commitment string
	Value      string
	Stub       []string
}

type Node struct {
	parent *VKTNode
	index  uint
}

func NewVKT(rootCommitment string, datastore ds.Datastore) *VKT {
	return GetVKT(rootCommitment, datastore)
}

func GetVKT(rootCommitment string, datastore ds.Datastore) *VKT {
	return (&VKT{}).init(rootCommitment, datastore)
}

func (t *VKT) init(rootCommitment string, datastore ds.Datastore) *VKT {
	n := (&VKTNode{}).init(rootCommitment, datastore)
	if n == nil {
		return nil
	}
	if rootCommitment == "" {
		n.updateCommitment(datastore)
	}

	t.Root = n
	return t
}

func (n *VKTNode) init(commitment string, datastore ds.Datastore) *VKTNode {
	n.children = make([]*VKTNode, 16)

	if commitment != "" {
		if datastore != nil {
			vktNodeKey := ds.NewKey(vktPrefix + fmt.Sprintf("/hash/%s", commitment))
			data, err := datastore.Get(context.Background(), vktNodeKey)
			if err != nil {
				log.Printf("Error getting value from datastore for key: %s %v\n", vktNodeKey, err)
				return nil
			} else {
				err := deserializeNode(string(data), n)
				if err != nil {
					return nil
				}
			}
		} else {
			log.Printf("[ERROR] datastore not provided!\n")
			return nil
		}
	}

	return n
}

func (n *VKTNode) PrintNode() {
	fmt.Println("----------")
	fmt.Println(n.Commitment)
	fmt.Println(" value:", n.value)
	fmt.Println(" children:", n.children)
	fmt.Println(" stub:", n.stub)
	fmt.Println("----------")
}

func (n *VKTNode) getChild(idx int64, datastore ds.Datastore) *VKTNode {
	if n.children[idx] != nil {
		return n.children[idx]
	} else if n.stub != nil && n.stub[idx] != "" {
		commitment := n.stub[idx]
		n.stub[idx] = ""
		if utils.AllEmpty(n.stub) {
			n.stub = nil
		}
		n.children[idx] = (&VKTNode{}).init(commitment, datastore)
		return n.children[idx]
	}
	return n.children[idx]
}

func (n *VKTNode) updateCommitment(datastore ds.Datastore) {
	n.Commitment = nodeCommitment(n)
	key := n.Commitment
	data := serializeNode(n)

	// save node data (deduped based on node hash/commitment)
	vktNodeKey := ds.NewKey(vktPrefix + fmt.Sprintf("/hash/%s", key))
	_, err := datastore.Get(context.Background(), vktNodeKey)
	if err != nil {
		// value does not exist for the provided key, so inserting..
		if err := datastore.Put(context.Background(), vktNodeKey, []byte(data)); err != nil {
			log.Println(fmt.Sprintf("Error setting data for key: %s", vktNodeKey), err)
			return
		}
	}
}

func serializeNode(n *VKTNode) string {
	stub := []string{}
	for _, v := range n.children {
		var commitment string
		if v != nil {
			commitment = v.Commitment
		} else {
			commitment = ""
		}
		stub = append(stub, commitment)
	}

	data := VKTNodeObj{Commitment: n.Commitment, Value: n.value, Stub: stub}
	encodedData, err := geth.RlpEncode(data)
	if err != nil {
		return ""
	}

	return encodedData
}

func deserializeNode(encodedData string, n *VKTNode) error {
	var data VKTNodeObj
	err := geth.RlpDecode(encodedData, &data)
	if err != nil {
		return err
	}

	n.Commitment = data.Commitment
	n.value = data.Value
	n.stub = data.Stub

	return nil
}

// TODO: Use Vector commitment (Pedersen commitment) at each node
func nodeCommitment(n *VKTNode) string {
	value := n.value
	for _, item := range n.children {
		if item != nil {
			value += item.Commitment
		}
	}
	return geth.Keccak256([]byte(value))
}

// NOTE: `key` is account address and must be a 32 byte hex string
func (t *VKT) Insert(key string, value string, datastore ds.Datastore) bool {
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
	node := t.Root

	for _, nibble := range path {
		idx, _ := strconv.ParseInt(string(nibble), 16, 32)
		if node.getChild(idx, datastore) == nil {
			node.children[idx] = (&VKTNode{}).init("", nil)
		}
		stack = append(stack, Node{parent: node, index: uint(idx)})
		node = node.getChild(idx, datastore)
	}

	node.value = value
	node.updateCommitment(datastore)

	for i := len(stack) - 1; i >= 0; i-- {
		parent := stack[i].parent
		parent.updateCommitment(datastore)
	}

	return true
}

// NOTE: `key` is account address and must be 32 byte hex string
func (t *VKT) Update(key string, value string, datastore ds.Datastore) bool {
	return t.Insert(key, value, datastore)
}

// NOTE: `key` is account address and must be 32 byte hex string
func (t *VKT) Get(key string, datastore ds.Datastore) string {
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
	node := t.Root

	for _, nibble := range path {
		idx, _ := strconv.ParseInt(string(nibble), 16, 32)
		n := node.getChild(idx, datastore)
		if n == nil {
			return "" // no item exists corresponding to the provided key
		}
		node = n
	}

	return node.value
}

// NOTE: `key` is account address and must be hex string
func (t *VKT) Delete(key string, datastore ds.Datastore) bool {
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
	node := t.Root

	for _, nibble := range path {
		idx, _ := strconv.ParseInt(string(nibble), 16, 32)
		n := node.getChild(idx, datastore)
		if n == nil {
			return false // no item exists corresponding to the provided key
		}
		stack = append(stack, Node{parent: node, index: uint(idx)})
		node = n
	}

	// remove the stored value
	node.value = ""
	node.updateCommitment(datastore)

	// prune unnecessary nodes, if any
	for i := len(stack) - 1; i >= 0; i-- {
		parent := stack[i].parent
		idx := stack[i].index
		child := parent.getChild(int64(idx), datastore)

		// Check if child has any children or value. If not, prune it
		if utils.AllNil(child.children) && (child.stub == nil || utils.AllEmpty(child.stub)) && child.value == "" {
			parent.children[idx] = nil // prune
			parent.updateCommitment(datastore)
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

// TODO: Implement getLeafValueStr() for strings larger than 32 bytes might need additional data chunks (additional leaf nodes)
// func getLeafValueStr(val float64) string {
// }

// func Demo(datastore ds.Datastore) {
// 	vkt := NewVKT(datastore)

// 	addr := "2ED69CD751722FC552bc8C521846d55f6BD8F090"

// 	key1 := GetNonceKey(addr)
// 	value1 := GetLeafValueInt(31) // nonce: 31
// 	success := vkt.Insert(key1, value1, datastore)
// 	fmt.Println("inserted:", success, vkt.Root.Commitment)

// 	value1 = GetLeafValueInt(32) // nonce: 32
// 	success = vkt.Update(key1, value1, datastore)
// 	fmt.Println("updated:", success, vkt.Root.Commitment)

// 	success = vkt.Delete(key1, datastore)
// 	fmt.Println("deleted:", success, vkt.Root.Commitment)

// 	value := vkt.Get(key1)
// 	if value != "" {
// 		fmt.Println("data:", value)
// 	} else {
// 		fmt.Println("Not found!")
// 	}

// 	key2 := GetBalanceKey(addr)
// 	value2 := GetLeafValueEthWei(2.8) // balance: 2.8 * 10^18 wei (2.8 eth)
// 	success = vkt.Insert(key2, value2, datastore)
// 	fmt.Println("inserted:", success, vkt.Root.Commitment)

// 	key3 := GetStorageKey(addr, 0)
// 	value3 := "hello"
// 	// TODO: Implement getLeafValueStr() for strings larger than 32 bytes might need additional data chunks (additional leaf nodes)
// 	success = vkt.Insert(key3, value3, datastore)
// 	fmt.Println("inserted:", success, vkt.Root.Commitment)

// 	fmt.Println("rootCommitment:", vkt.Root.Commitment)
// }
