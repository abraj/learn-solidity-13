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

	if includes := isValidator(from, validators); !includes {
		log.Println("[WARN] Invalid validator:", from)
		return false
	}

	// TODO: consider for current block only
	// TODO: assert valid `phase` value

	fmt.Println("~~~")
	fmt.Println("msg.Data:", data, from)

	return true
}

func isValidator(peerID peer.ID, validators []peer.ID) bool {
	includes := false
	for _, v := range validators {
		if v.String() == peerID.String() {
			includes = true
		}
	}
	return includes
}

func nextBlockInfo() (int, int) {
	var nextBlockNumber int
	var waitTimeMsec int

	slotNumber, timeLeftMsec := shared.SlotInfo()
	if timeLeftMsec > (shared.SLOT_DURATION - shared.BUFFER_TIME) {
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

func consumePhase1(msg *pubsub.Message) {
	data := string(msg.Data)
	from := msg.GetFrom()

	fmt.Println("<-----")
	fmt.Println("Received message:", data, from)
}

func listenPhase1(topic *pubsub.Topic) {
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

			consumePhase1(msg)
		}
	}()
}

func InitConsensusLoop(node host.Host, validators []peer.ID, ps *pubsub.PubSub) {
	if includes := isValidator(node.ID(), validators); !includes {
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
	listenPhase1(topic)

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
