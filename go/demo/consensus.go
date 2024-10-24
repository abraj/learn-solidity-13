package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"libp2pdemo/shared"
	"libp2pdemo/utils"
	"log"
	"math/rand"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

type Phase1Payload struct {
	Block      int    `json:"block"`
	Phase      int    `json:"phase"`
	Hash       string `json:"hash"`
	Ciphertext string `json:"ciphertext"`
}

const consensusTopic = "baadal-consensus"

// NOTE: The validator function is called while receiving (non-local) messages
// as well as _sending_ messages.
func payloadDataValidator(msg *pubsub.Message, validators []peer.ID) bool {
	if len(msg.Data) == 0 {
		log.Println("[WARN] empty message")
		return false
	}
	if len(msg.Data) > 1024 {
		log.Println("[WARN] message size exceeds 1KB")
		return false
	}

	data := string(msg.Data)
	from := msg.GetFrom()

	if includes := utils.IsValidator(from, validators); !includes {
		log.Printf("[WARN] Invalid validator: %s\n", from)
		return false
	}

	var payload Phase1Payload
	err := json.Unmarshal([]byte(data), &payload)
	if err != nil {
		log.Println("Error unmarshalling JSON:", err)
		return false
	}

	slotNumber, timeLeftMsec := shared.SlotInfo()
	cond1 := payload.Block == slotNumber
	cond2 := payload.Block == slotNumber+1 && timeLeftMsec < shared.MAX_CLOCK_DRIFT/2
	if !cond1 && !cond2 {
		log.Printf("Invalid block number: %d (%d, %d)\n", payload.Block, slotNumber, timeLeftMsec)
		return false
	}

	if payload.Phase != 1 {
		log.Printf("Invalid phase value: %d (%d)\n", payload.Phase, payload.Block)
		return false
	}

	return true
}

func nextBlockInfo() (int, int) {
	var nextBlockNumber int
	var waitTimeMsec int

	slotNumber, timeLeftMsec := shared.SlotInfo()
	timeElapsed := shared.SLOT_DURATION - timeLeftMsec
	if timeElapsed < shared.MAX_LAG {
		// submit for this block, you're already late!
		nextBlockNumber = slotNumber
		waitTimeMsec = 0
	} else {
		// schedule for next block
		nextBlockNumber = slotNumber + 1
		waitTimeMsec = timeLeftMsec
	}

	if nextBlockNumber <= shared.GetLatestBlockNumber() {
		return 0, timeLeftMsec + shared.SLOT_DURATION
	}
	return nextBlockNumber, waitTimeMsec
}

func createPhase1Payload(blockNumber int, entropy string) string {
	hash := utils.CreateSHA3Hash(entropy)

	key := utils.RandomHex(32)
	iv := utils.RandomHex(16)

	message := entropy
	ciphertext, err := utils.EncryptAES256CBC(key, iv, message)
	if err != nil {
		panic(err)
	}

	payload := Phase1Payload{Block: blockNumber, Phase: 1, Hash: hash, Ciphertext: ciphertext}
	payloadData, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	payloadStr := string(payloadData)
	return payloadStr
}

// phase1: commit phase
func triggerPhase1(nextBlockNumber int, payload string, topic *pubsub.Topic) {
	shared.SetLatestBlockNumber(nextBlockNumber)
	topic.Publish(context.Background(), []byte(payload))
}

func consumePhase1(payload Phase1Payload, from peer.ID) {
	// SHA3: [64 len hex] 32 bytes (256 bits) hash
	if len(payload.Hash) != 64 {
		log.Printf("Invalid hash: %s (%s)\n", payload.Hash, from)
		return
	}

	// AES-256: [160 len hex] 80 bytes: 64 bytes (plaintext as 64 length utf8) encypted text + 16 bytes IV
	if len(payload.Ciphertext) != 160 {
		log.Printf("Invalid ciphertext: %s (%s)\n", payload.Ciphertext, from)
		return
	}

	fmt.Println("<-----")
	fmt.Printf("[%d:%s] %s %s\n", payload.Block, from, payload.Hash, payload.Ciphertext)
}

func listenConsensusTopic(topic *pubsub.Topic) {
	subs, err := topic.Subscribe()
	if err != nil {
		panic(err)
	}

	go func() {
		defer subs.Cancel()
		for {
			msg, err := subs.Next(context.Background())
			if err != nil {
				log.Println("Error receiving message:", err)
				return
			}

			data := string(msg.Data)
			from := msg.GetFrom()

			var payload Phase1Payload
			err = json.Unmarshal([]byte(data), &payload)
			if err != nil {
				log.Println("Error unmarshalling JSON:", err)
				return
			}

			if payload.Phase == 1 {
				consumePhase1(payload, from)
			} else {
				log.Fatalf("[ERROR] Unhandled phase value: %d (%d)\n", payload.Phase, payload.Block)
			}
		}
	}()
}

func InitConsensusLoop(node host.Host, validators []peer.ID, ps *pubsub.PubSub) {
	if includes := utils.IsValidator(node.ID(), validators); !includes {
		// current node is not a validator
		return
	}

	payloadValidator := func(ctx context.Context, pid peer.ID, msg *pubsub.Message) bool {
		// NOTE: `pid` could be an intermediary or relaying node, not necessarily the original publisher
		// unlike `msg.GetFrom()` which is the peer ID of the original publisher of the message
		return payloadDataValidator(msg, validators)
	}
	err := ps.RegisterTopicValidator(consensusTopic, payloadValidator)
	if err != nil {
		panic(err)
	}
	topic, err := ps.Join(consensusTopic)
	if err != nil {
		panic(err)
	}
	listenConsensusTopic(topic)

	go func() {
		_, initialDelayMsec := nextBlockInfo()
		additionalDelay := shared.SLOT_DURATION
		delay := initialDelayMsec + additionalDelay
		time.Sleep(time.Duration(delay) * time.Millisecond)

		for {
			midpointDelay := shared.SLOT_DURATION / 2 // drift to slot midpoint
			randomDelay := int(rand.Float64() * 1000) // 0-1s
			delay := midpointDelay + randomDelay
			time.Sleep(time.Duration(delay) * time.Millisecond)

			nextBlockNumber, waitTimeMsec := nextBlockInfo()
			if nextBlockNumber > 0 {
				ticker := time.After(time.Duration(waitTimeMsec) * time.Millisecond)
				entropy := utils.RandomHex(32)
				payload := createPhase1Payload(nextBlockNumber, entropy)
				<-ticker
				triggerPhase1(nextBlockNumber, payload, topic)
			} else {
				time.Sleep(time.Duration(waitTimeMsec) * time.Millisecond)
			}
		}
	}()
}
