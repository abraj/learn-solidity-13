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

	ds "github.com/ipfs/go-datastore"
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

type Phase1Data struct {
	Hash       string `json:"hash"`
	Ciphertext string `json:"ciphertext"`
}

type Channels struct {
	quorum chan bool
}

const consensusTopic = "baadal-consensus"
const consensusPrefix = "/consensus/block"

func channelsFactory(channelsMap map[string]Channels, key string) Channels {
	if channelsMap[key].quorum == nil {
		timer := time.NewTimer(shared.PHASE_DURATION * time.Millisecond)
		quorum := make(chan bool)

		// mutex not needed (since no parallel goroutines)
		channelsMap[key] = Channels{quorum}

		go func() {
			defer timer.Stop()
			defer close(quorum)
			select {
			case <-quorum:
				// quorum achieved
				fmt.Println("triggerPhase2..")
			case <-timer.C:
				// timeout
			}
		}()
	}
	return channelsMap[key]
}

// NOTE: The validator function is called while receiving (non-local) messages
// as well as _sending_ messages.
func payloadDataValidator(ctx context.Context, pid peer.ID, msg *pubsub.Message) bool {
	// NOTE: `pid` could be an intermediary or relaying node, not necessarily the original publisher
	// unlike `msg.GetFrom()` which is the peer ID of the original publisher of the message

	if len(msg.Data) == 0 {
		log.Println("[WARN] empty message")
		return false
	}
	if len(msg.Data) > 1024 {
		log.Println("[WARN] message size exceeds 1KB")
		return false
	}

	data := string(msg.Data)
	// from := msg.GetFrom()

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
func triggerPhase1(nextBlockNumber int, data string, topic *pubsub.Topic, validators []peer.ID, datastore ds.Datastore) {
	shared.SetLatestBlockNumber(nextBlockNumber)

	// var payload Phase1Payload
	// err := json.Unmarshal([]byte(data), &payload)
	// if err != nil {
	// 	log.Println("Error unmarshalling JSON:", err)
	// 	return
	// }

	validatorsList, err := json.Marshal(validators)
	if err != nil {
		log.Println("Error marshalling validators list:", err)
		return
	}
	validatorsListStr := string(validatorsList)

	validatorsKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/validators", nextBlockNumber))
	if err := datastore.Put(context.Background(), validatorsKey, []byte(validatorsListStr)); err != nil {
		log.Println(fmt.Sprintf("Error setting data for key: %s", validatorsKey), err)
		return
	}

	err = topic.Publish(context.Background(), []byte(data))
	if err != nil {
		panic(err)
	}
}

func consumePhase1(payload Phase1Payload, from peer.ID, datastore ds.Datastore, channels Channels) {
	validatorsKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/validators", payload.Block))
	validatorsList, err := datastore.Get(context.Background(), validatorsKey)
	if err != nil {
		// log.Println(fmt.Sprintf("Error getting data for key: %s", validatorsKey), err)
		return
	}
	// validatorsListStr := string(validatorsList)

	validators := make([]peer.ID, 0)
	err = json.Unmarshal(validatorsList, &validators)
	if err != nil {
		log.Println("Error unmarshalling JSON:", err)
		return
	}

	if includes := utils.IsValidator(from, validators); !includes {
		log.Printf("[WARN] Invalid validator: %s\n", from)
		return
	}

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

	data := Phase1Data{Hash: payload.Hash, Ciphertext: payload.Ciphertext}
	p1Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	p1DataStr := string(p1Data)
	p1Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	if err := datastore.Put(context.Background(), p1Key, []byte(p1DataStr)); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", p1Key), err)
		return
	}

	prefix := consensusPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, _, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	fmt.Println("<---- prefix:", len(validators), len(keys), prefix)
	// fmt.Println("keys:", keys)
	// fmt.Println("values:", values)

	// quorum: 3/4 majority
	if len(keys)*4 >= len(validators)*3 {
		channels.quorum <- true
	}

	// TODO: cleanup memory store, datastore, etc. for each /block-number
}

func listenConsensusTopic(topic *pubsub.Topic, datastore ds.Datastore) {
	subs, err := topic.Subscribe()
	if err != nil {
		panic(err)
	}

	go func(datastore ds.Datastore) {
		defer subs.Cancel()

		channelsMap := make(map[string]Channels)
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
				// keep it outside goroutine for thread-safety of map
				key := fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
				channels := channelsFactory(channelsMap, key)

				go func() {
					consumePhase1(payload, from, datastore, channels)
				}()
			} else {
				log.Fatalf("[ERROR] Unhandled phase value: %d (%d)\n", payload.Phase, payload.Block)
			}
		}
	}(datastore)
}

func InitConsensusLoop(node host.Host, validators []peer.ID, ps *pubsub.PubSub, datastore ds.Datastore) {
	if includes := utils.IsValidator(node.ID(), validators); !includes {
		// current node is not a validator
		return
	}

	err := ps.RegisterTopicValidator(consensusTopic, payloadDataValidator)
	if err != nil {
		panic(err)
	}
	topic, err := ps.Join(consensusTopic)
	if err != nil {
		panic(err)
	}
	listenConsensusTopic(topic, datastore)

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
				data := createPhase1Payload(nextBlockNumber, entropy)
				<-ticker
				go func() {
					triggerPhase1(nextBlockNumber, data, topic, validators, datastore)
				}()
			} else {
				time.Sleep(time.Duration(waitTimeMsec) * time.Millisecond)
			}
		}
	}()
}
