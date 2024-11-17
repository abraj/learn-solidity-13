package demo

import (
	"encoding/hex"
	"fmt"
	"libp2pdemo/shared"
	"log"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/host"
)

type BlockHeader struct {
	BlockNumber   int64  // the slot the block belongs to (block number)
	ParentHash    string // Hash of the previous block
	ProposerIndex int64  // index of the validator (block proposer) in the validator registry
	Timestamp     int64  // timestamp for the block
	StateRoot     string // Merkle root representing the state at the time of the block
	// Signature   (v, r, s)  // Proposer\'s BLS signature
}

func InitBlock(node host.Host, datastore ds.Datastore) {
	time.Sleep(2 * time.Second)

	slotNumber, timeLeftMsec := shared.SlotInfo()
	timer := time.NewTimer(time.Duration(timeLeftMsec) * time.Millisecond)
	defer timer.Stop()

	blockNumber := int64(slotNumber)
	parentHash := "0x" + hex.EncodeToString(make([]byte, 32))
	timestamp := shared.NetworkTime()
	stateRoot := getBeaconStateRoot()

	if stateRoot == "" {
		log.Printf("[WARN] state root missing!\n")
	}

	validatorRegistry := GetValidatorRegistry()

	isValidator := false
	var proposerIndex int64
	for idx, validatorInfo := range validatorRegistry {
		if validatorInfo.ID == node.ID().String() {
			isValidator = true
			proposerIndex = int64(idx)
		}
	}

	if !isValidator {
		log.Printf("[Unexpected] Not a validator! %s\n", node.ID().String())
		return
	}

	header := BlockHeader{BlockNumber: blockNumber, ParentHash: parentHash, ProposerIndex: proposerIndex, Timestamp: timestamp, StateRoot: stateRoot}

	<-timer.C
	fmt.Println("----")
	fmt.Printf(">> header: %+v\n", header)
	fmt.Println("----")
}
