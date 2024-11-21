package demo

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"libp2pdemo/shared"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	ds "github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/host"
)

type BlockHeader struct {
	Timestamp     uint64 // timestamp for the block
	BlockNumber   uint64 // the slot the block belongs to (block number)
	ParentHash    string // Hash of the previous block
	ProposerIndex uint64 // index of the validator (block proposer) in the validator registry
	StateRoot     string // Merkle root representing the state at the time of the block
	BodyRoot      string // Merkle root representing the block body
	Signature     string // Proposer's signature (ideally BLS)
	// ExecutionPayloadHeader ExecutionPayloadHeader
}

type BlockHeaderData struct {
	Timestamp     uint64
	BlockNumber   uint64
	ParentHash    string
	ProposerIndex uint64
	StateRoot     string
	BodyRoot      string
}

type BlockBody struct {
	// DepositsRoot: Merkle root representing all the deposits into the deposit contract
	// Deposits: list of new deposits to the deposit contract
	// ProposerSlashings: list of slashing events for other proposers (validators)
	// AttesterSlashings: list of slashing events for validators during the attestation process
	// VoluntaryExits: list of validators exiting the network
	// Attestations: [Validator ID, Vote Signature, ..] list of validator votes confirming the validity of earlier blocks and checkpoints
	ExecutionPayload ExecutionPayload
}

type ExecutionPayloadHeader struct {
	// FeeRecipient	account address for paying transaction fees and rewards
	// GasLimit	maximum gas allowed in this block
	// GasUsed	the actual amount of gas used in this block
	// BaseFeePerGas	the base fee value
	// LogsBloom	data structure containing event logs
	// ReceiptsRoot	Merkle root of all transaction receipts in a block
	// TransactionsRoot	Merkle root of all transactions in the block
	// WithdrawalRoot	Merkle root of all stake withdrawals
}

type ExecutionPayload struct {
	Transactions []TransactionData // list of all transactions included in the block
	// Withdrawals	list of stake withdrawals
	// 	ValidatorIndex	validator index value
	// 	Address	account address that has withdrawn
	// 	Amount	withdrawal amount
	// 	Index	withdrawal index value
}

type TransactionData struct {
	// 	tx_hash: "0x...",                          // Transaction hash
	// 	from: "0x...",                             // Sender address
	// 	to: "0x...",                               // Recipient address
	// 	value: "1000000000000000000",              // Transaction value (in wei)
	// 	gas: 21000,                                // Gas used by the transaction
	// 	gas_price: "1000000000",                   // Gas price (in wei)
	// 	nonce: 1                                   // Nonce
	// 	Signature: "v, r, s"                       // The sender's signature (ECDSA)
}

func InitBlock(node host.Host, datastore ds.Datastore) {
	time.Sleep(2 * time.Second)

	slotNumber, timeLeftMsec := shared.SlotInfo()
	timer := time.NewTimer(time.Duration(timeLeftMsec) * time.Millisecond)
	defer timer.Stop()

	timestamp := shared.NetworkTime()
	blockNumber := slotNumber
	parentHash := "0x" + hex.EncodeToString(make([]byte, 32))
	stateRoot := getBeaconStateRoot()
	bodyRoot := getBlockBodyRoot()

	if stateRoot == "" {
		log.Printf("[WARN] state root missing!\n")
	}

	validatorRegistry := GetValidatorRegistry()

	isValidator := false
	var proposerIndex uint64
	for idx, validatorInfo := range validatorRegistry {
		if validatorInfo.ID == node.ID().String() {
			isValidator = true
			proposerIndex = uint64(idx)
		}
	}

	if !isValidator {
		log.Printf("[Unexpected] Not a validator! %s\n", node.ID().String())
		return
	}

	privKey := shared.GetPrivateKey()
	if privKey == nil {
		log.Fatalf("[ERROR] private key should be set by now!\n")
	}

	headerData := BlockHeaderData{
		BlockNumber: 123,

		// Timestamp:     timestamp,
		// BlockNumber: blockNumber,
		// ParentHash:    parentHash,
		// ProposerIndex: proposerIndex,
		// StateRoot:     stateRoot,
		// BodyRoot:      bodyRoot,
	}

	fmt.Println("1..")
	// data, err := geth.RlpEncode(headerData)
	// if err != nil {
	// 	return
	// }

	var buffer bytes.Buffer
	err := rlp.Encode(&buffer, headerData)
	if err != nil {
		log.Printf("Failed to RLP encode: %+v\n", headerData)
		return
	}
	data := hex.EncodeToString(buffer.Bytes())

	fmt.Println("2..")

	// TODO: ideally BLS signature should be used (using ECDSA signature for now)
	signBytes, err := privKey.Sign([]byte(data))
	if err != nil {
		log.Printf("[ERROR] Unable to create signature!\n")
		return
	}
	sign := "0x" + hex.EncodeToString(signBytes)

	header := BlockHeader{
		Timestamp:     timestamp,
		BlockNumber:   blockNumber,
		ParentHash:    parentHash,
		ProposerIndex: proposerIndex,
		StateRoot:     stateRoot,
		BodyRoot:      bodyRoot,
		Signature:     sign,
	}
	// proposer := validatorRegistry[proposerIndex]

	<-timer.C
	fmt.Println("----")
	fmt.Printf(">> header: %+v\n", header)
	fmt.Println("----")
}

func getBlockBodyRoot() string {
	executionPayload := ExecutionPayload{Transactions: make([]TransactionData, 0)}
	blockBody := BlockBody{ExecutionPayload: executionPayload}

	blockBodyRoot := getMerkleRoot(blockBody)
	return blockBodyRoot
}
