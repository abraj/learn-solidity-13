package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"libp2pdemo/shared"
	"libp2pdemo/utils"
	"log"
	"math/rand"
	"sync"
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
	Delayed    bool   `json:"delayed"`
}

type Phase1Status struct {
	PhaseCompleted int `json:"phase"`
}

type Channels struct {
	mutex  *sync.Mutex
	quorum chan int
}

const consensusTopic = "baadal-consensus"
const consensusPrefix = "/consensus/block"

func channelsFactory(key string, channelsMap map[string]Channels, datastore ds.Datastore) Channels {
	if channelsMap[key].quorum == nil {
		timer := time.NewTimer(shared.PHASE_DURATION * time.Millisecond)

		// var mutex sync.Mutex
		mutex := sync.Mutex{}
		quorum := make(chan int)

		// mutex not needed (since no parallel goroutines)
		channelsMap[key] = Channels{mutex: &mutex, quorum: quorum}

		go func() {
			defer timer.Stop()
			defer close(quorum)
			select {
			case block := <-quorum:
				// quorum achieved
				go func() {
					triggerPhase2(block, datastore)
				}()
			case <-timer.C:
				// timeout
				fmt.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>> timeout")
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

	statusKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/status", nextBlockNumber))
	if err := datastore.Put(context.Background(), statusKey, []byte("{}")); err != nil {
		log.Println(fmt.Sprintf("Error setting data for key: %s", statusKey), err)
		return
	}

	err = topic.Publish(context.Background(), []byte(data))
	if err != nil {
		panic(err)
	}
}

// phase2: transition (commit-close) phase
func triggerPhase2(block int, datastore ds.Datastore) {
	fmt.Println("triggerPhase2..", block, datastore)
}

func getStatusObj(block int, datastore ds.Datastore) (Phase1Status, error) {
	statusKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/status", block))
	statusData, err := datastore.Get(context.Background(), statusKey)
	if err != nil {
		log.Println(fmt.Sprintf("Error getting data for key: %s", statusKey), err)
		return Phase1Status{}, err
	}

	var statusObj Phase1Status
	err = json.Unmarshal(statusData, &statusObj)
	if err != nil {
		log.Println("Error unmarshalling status data:", err)
		return Phase1Status{}, err
	}

	return statusObj, nil
}

func consumePhase1(payload Phase1Payload, from peer.ID, datastore ds.Datastore, channels Channels) {
	validatorsKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/validators", payload.Block))
	validatorsList, err := datastore.Get(context.Background(), validatorsKey)
	if err != nil {
		log.Println(fmt.Sprintf("Error getting data for key: %s", validatorsKey), err)
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

	channels.mutex.Lock()

	statusObj, err := getStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}

	phase1Completed := statusObj.PhaseCompleted >= 1

	data := Phase1Data{Hash: payload.Hash, Ciphertext: payload.Ciphertext, Delayed: phase1Completed}
	p1Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// move below to avoid capturing laggard/delayed messages
	p1DataStr := string(p1Data)
	p1Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	if err := datastore.Put(context.Background(), p1Key, []byte(p1DataStr)); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", p1Key), err)
		return
	}

	// move above to avoid capturing laggard/delayed messages
	if phase1Completed {
		// log.Printf("Phase 1 already completed. Status: %d\n", statusObj.PhaseCompleted)
		return
	}

	// query data needed for quorum computation
	prefix := consensusPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, _, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	fmt.Println("<---- prefix:", len(validators), len(keys), prefix, from)
	// fmt.Println("keys:", keys)
	// fmt.Println("values:", values)

	// wait for 90th percentile response
	if len(keys)*10 >= len(validators)*9 {
		statusObj.PhaseCompleted = 1
		statusData, err := json.Marshal(statusObj)
		if err != nil {
			log.Println("Error marshalling status data:", err)
			return
		}

		statusKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/status", payload.Block))
		if err := datastore.Put(context.Background(), statusKey, statusData); err != nil {
			log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", statusKey), err)
			return
		}

		// close phase1
		channels.quorum <- payload.Block
	}

	channels.mutex.Unlock()

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
				channels := channelsFactory(key, channelsMap, datastore)

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
