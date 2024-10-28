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

func nextBlockInfo() (int, int) {
	var nextBlockNumber int
	var waitTimeMsec int

	slotNumber, timeLeftMsec := shared.SlotInfo()
	if timeLeftMsec > (shared.SLOT_DURATION - shared.BUFFER_TIME) {
		// submit for this block
		nextBlockNumber = slotNumber
		waitTimeMsec = 0
	} else {
		// schedule for next block
		nextBlockNumber = slotNumber + 1
		waitTimeMsec = timeLeftMsec
	}

	if nextBlockNumber <= shared.GetLatestBlockNumber() {
		return 0, shared.SLOT_DURATION / 2
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

func listenPhase1(topic *pubsub.Topic) {
	subs, err := topic.Subscribe()
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			msg, err := subs.Next(context.Background())
			if err != nil {
				log.Println("Error receiving message:", err)
				return
			}

			fmt.Println("------")
			fmt.Printf("Received message: %s from %s\n", msg.Data, msg.GetFrom())
		}
	}()
}

func InitConsensusLoop(node host.Host, validators []peer.ID, ps *pubsub.PubSub) {
	// err := ps.RegisterTopicValidator(consensusTopic, payloadValidator)
	// if err != nil {
	// 	panic(err)
	// }
	topic, err := ps.Join(consensusTopic)
	if err != nil {
		panic(err)
	}
	listenPhase1(topic)

	includes := false
	for _, v := range validators {
		if v.String() == node.ID().String() {
			includes = true
		}
	}

	if !includes {
		return
	}

	go func() {
		time.Sleep(10 * time.Second)

		for {
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
			duration := shared.BUFFER_TIME + int(rand.Float64()*1000)
			time.Sleep(time.Duration(duration) * time.Millisecond)
		}
	}()
}
