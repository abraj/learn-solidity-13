package demo

import (
	"encoding/hex"
	"fmt"
	"libp2pdemo/vkt"
	"log"

	ds "github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/host"
)

type BeaconState struct {
	ValidatorRegistry []ValidatorInfo
	// EpochInfo
	//   # current epoch committee
	//   # next epoch committee
	// # Finality Checkpoints
	// RandaoReveal/RandaoMix
	// ProtocolInfo
	//   # Other protocol-specific data
	ExecutionPayloadState ExecutionPayloadState
}

type ValidatorInfo struct {
	ID        string
	PublicKey string
	// StakedBalance  int64
	// Status          string // (active, exited, slashed, etc.)
	// ActivationEpoch int64
	// ExitEpoch       int64
	// SlashingStatus  []string
}

type ExecutionPayloadState struct {
	ExecutionStateRoot string
}

var execState *vkt.VKT
var beaconState *BeaconState // in-memory beaconState (for now)

func InitState(node host.Host, datastore ds.Datastore) {
	execState = initExecutionState(datastore)
	beaconState = initBeaconState(node)
}

func initValidatorRegistry(node host.Host) []ValidatorInfo {
	var validatorRegistry []ValidatorInfo

	// TODO: get "current" validator list using activation and exit epochs
	validators := GetValidatorsList()

	for _, peerID := range validators {
		var pubKey string
		pubKey0 := node.Peerstore().PubKey(peerID)
		if pubKey0 == nil {
			log.Printf("[WARN] Could not extract public key from peerID: %s\n", peerID.String())
		} else {
			pubKeyBytes, err := pubKey0.Raw()
			if err != nil {
				log.Printf("[ERROR] Could not extract raw bytes from public key: %s\n", peerID.String())
			} else {
				pubKey = "0x" + hex.EncodeToString(pubKeyBytes)
			}
		}
		validatorInfo := ValidatorInfo{ID: peerID.String(), PublicKey: pubKey}
		validatorRegistry = append(validatorRegistry, validatorInfo)
	}

	if len(validatorRegistry) != len(validators) {
		log.Printf("[ERROR] Unexpected! validatorRegistry length (%d) must match validators length (%d)\n", len(validatorRegistry), len(validators))
		return nil
	}

	return validatorRegistry
}

func initExecutionPayloadState() ExecutionPayloadState {
	executionStateRoot := getExecutionStateRoot()
	executionPayloadState := ExecutionPayloadState{ExecutionStateRoot: executionStateRoot}
	return executionPayloadState
}

func initBeaconState(node host.Host) *BeaconState {
	validatorRegistry := initValidatorRegistry(node)
	executionPayloadState := initExecutionPayloadState()
	if validatorRegistry == nil || executionPayloadState.ExecutionStateRoot == "" {
		return nil
	}

	beaconState := BeaconState{ValidatorRegistry: validatorRegistry, ExecutionPayloadState: executionPayloadState}
	return &beaconState
}

func GetValidatorRegistry() []ValidatorInfo {
	if beaconState == nil {
		return nil
	}
	beaconStateValue := *beaconState
	return beaconStateValue.ValidatorRegistry
}

func getBeaconStateRoot() string {
	if beaconState == nil {
		return ""
	}
	beaconStateValue := *beaconState

	beaconStateRoot := getMerkleRoot(beaconStateValue)

	return beaconStateRoot
}

func getExecutionStateRoot() string {
	if execState == nil {
		return ""
	}
	return "0x" + execState.Root.Commitment
}

func initExecutionState(datastore ds.Datastore) *vkt.VKT {
	// state := vkt.NewVKT("", datastore)

	// rootCommitment := "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"
	rootCommitment := "594bf3ce1879ba15a40a6a1e9fd6cc7557af274de8ed62956629e40a929bd37b"
	// rootCommitment := "6c4210277f10b84c83926787c54a4d2646f347d8ca2da10ebfda2b6be4436b5f"
	state := vkt.GetVKT(rootCommitment, datastore)

	if state == nil {
		return nil
	}

	addr := "2ED69CD751722FC552bc8C521846d55f6BD8F090"

	key1 := vkt.GetNonceKey(addr)
	// value1 := vkt.GetLeafValueInt(31) // nonce: 31
	// success := state.Insert(key1, value1, datastore)
	// fmt.Println("inserted:", success, state.Root.Commitment)

	// value1 = vkt.GetLeafValueInt(32) // nonce: 32
	// success = state.Update(key1, value1, datastore)
	// fmt.Println("updated:", success, state.Root.Commitment)

	// success = state.Delete(key1, datastore)
	// fmt.Println("deleted:", success, state.Root.Commitment)

	value := state.Get(key1, datastore)
	if value != "" {
		fmt.Println("data:", value)
	} else {
		fmt.Println("Not found!")
	}

	fmt.Println("[exec] state root:", state.Root.Commitment)
	return state
}
