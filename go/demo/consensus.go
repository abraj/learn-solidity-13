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

type PhasePayload struct {
	From  string `json:"from"`
	Block int    `json:"block"`
	Phase int    `json:"phase"`
}

type Phase1Payload struct {
	From       string `json:"from"`
	Block      int    `json:"block"`
	Phase      int    `json:"phase"`
	Hash       string `json:"hash"`
	Ciphertext string `json:"ciphertext"`
}

type Phase3Payload struct {
	From    string `json:"from"`
	Block   int    `json:"block"`
	Phase   int    `json:"phase"`
	Entropy string `json:"entropy"`
}

type PhaseData struct {
	Delayed bool `json:"delayed"`
}

type Phase1Data struct {
	Hash       string `json:"hash"`
	Ciphertext string `json:"ciphertext"`
	Delayed    bool   `json:"delayed"`
}

type Phase3Data struct {
	Entropy string `json:"entropy"`
	Delayed bool   `json:"delayed"`
}

type PhaseStatus struct {
	Entropy        string `json:"entropy"`
	PhaseCompleted int    `json:"phase"`
}

type Channels struct {
	mutex  *sync.Mutex
	quorum chan int
}

const consensusTopic = "baadal-consensus"
const consensusPrefix = "/consensus/block"

func channelsFactory(key string, channelsMap map[string]Channels, onQuorum func(block int)) Channels {
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
					onQuorum(block)
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
func payloadDataValidator(msg *pubsub.Message) bool {
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

	var payload PhasePayload
	err := json.Unmarshal([]byte(data), &payload)
	if err != nil {
		log.Println("Error unmarshalling JSON:", err)
		return false
	}

	if from.String() != payload.From {
		log.Printf("Unable to verify sender: %s %s\n", from, payload.From)
		return false
	}

	slotNumber, timeLeftMsec := shared.SlotInfo()
	cond1 := payload.Block == slotNumber
	cond2 := payload.Block == slotNumber+1 && timeLeftMsec < shared.MAX_CLOCK_DRIFT/2
	if !cond1 && !cond2 {
		log.Printf("Invalid block number: %d (%d, %d)\n", payload.Block, slotNumber, timeLeftMsec)
		return false
	}

	if payload.Phase != 1 && payload.Phase != 2 && payload.Phase != 3 {
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

func createPhase1Payload(node host.Host, blockNumber int, entropy string) string {
	hash := utils.CreateSHA3Hash(entropy)

	key := utils.RandomHex(32)
	iv := utils.RandomHex(16)

	message := entropy
	ciphertext, err := utils.EncryptAES256CBC(key, iv, message)
	if err != nil {
		panic(err)
	}

	from := node.ID().String()
	payload := Phase1Payload{From: from, Block: blockNumber, Phase: 1, Hash: hash, Ciphertext: ciphertext}
	payloadData, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	payloadStr := string(payloadData)
	return payloadStr
}

func createPhase2Payload(node host.Host, blockNumber int) string {
	from := node.ID().String()
	payload := PhasePayload{From: from, Block: blockNumber, Phase: 2}
	payloadData, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	payloadStr := string(payloadData)
	return payloadStr
}

func createPhase3Payload(node host.Host, blockNumber int, datastore ds.Datastore) string {
	statusObj, err := getStatusObj(blockNumber, datastore)
	if err != nil {
		return ""
	}

	from := node.ID().String()
	payload := Phase3Payload{From: from, Block: blockNumber, Phase: 3, Entropy: statusObj.Entropy}
	payloadData, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	payloadStr := string(payloadData)
	return payloadStr
}

// phase1: commit phase
func triggerPhase1(entropy string, nextBlockNumber int, data string, topic *pubsub.Topic, validators []peer.ID, datastore ds.Datastore) {
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

	statusObj := PhaseStatus{Entropy: entropy, PhaseCompleted: 0}
	err = setStatusObj(statusObj, nextBlockNumber, datastore)
	if err != nil {
		return
	}

	if data == "" {
		return
	}

	err = topic.Publish(context.Background(), []byte(data))
	if err != nil {
		log.Printf("Could not publish invalid message: %v\n", err)
		// panic(err)
	}
}

// phase2: transition (commit-close) phase
func triggerPhase2(block int, node host.Host, topic *pubsub.Topic) {
	fmt.Println("triggerPhase2..", block)

	data := createPhase2Payload(node, block)
	if data == "" {
		return
	}

	err := topic.Publish(context.Background(), []byte(data))
	if err != nil {
		log.Printf("Could not publish invalid message: %v\n", err)
		// panic(err)
	}
}

// phase3: reveal phase
func triggerPhase3(block int, node host.Host, topic *pubsub.Topic, datastore ds.Datastore) {
	fmt.Println("triggerPhase3..", block)

	data := createPhase3Payload(node, block, datastore)
	if data == "" {
		return
	}

	err := topic.Publish(context.Background(), []byte(data))
	if err != nil {
		log.Printf("Could not publish invalid message: %v\n", err)
		// panic(err)
	}
}

// phase4: tally phase
func triggerPhase4(block int, node host.Host, topic *pubsub.Topic) {
	fmt.Println("triggerPhase4..", block)
}

func getStatusObj(block int, datastore ds.Datastore) (PhaseStatus, error) {
	statusKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/status", block))
	statusData, err := datastore.Get(context.Background(), statusKey)
	if err != nil {
		log.Println(fmt.Sprintf("Error getting data for key: %s", statusKey), err)
		return PhaseStatus{}, err
	}

	var statusObj PhaseStatus
	err = json.Unmarshal(statusData, &statusObj)
	if err != nil {
		log.Println("Error unmarshalling status data:", err)
		return PhaseStatus{}, err
	}

	return statusObj, nil
}

func setStatusObj(statusObj PhaseStatus, block int, datastore ds.Datastore) error {
	statusData, err := json.Marshal(statusObj)
	if err != nil {
		log.Println("Error marshalling status data:", err)
		return err
	}

	statusKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/status", block))
	if err := datastore.Put(context.Background(), statusKey, statusData); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", statusKey), err)
		return err
	}

	return nil
}

func getValidatorsFromStore(payload PhasePayload, datastore ds.Datastore) ([]peer.ID, error) {
	validatorsKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/validators", payload.Block))
	validatorsList, err := datastore.Get(context.Background(), validatorsKey)
	if err != nil {
		log.Println(fmt.Sprintf("Error getting data for key: %s", validatorsKey), err)
		return nil, err
	}
	// validatorsListStr := string(validatorsList)

	validators := make([]peer.ID, 0)
	err = json.Unmarshal(validatorsList, &validators)
	if err != nil {
		log.Println("Error unmarshalling JSON:", err)
		return nil, err
	}

	return validators, nil
}

func consumePhase1(payload Phase1Payload, from peer.ID, datastore ds.Datastore, channels Channels) {
	_payload := PhasePayload{From: payload.From, Block: payload.Block, Phase: payload.Block}
	validators, err := getValidatorsFromStore(_payload, datastore)
	if err != nil {
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

	// query data needed for deciding progress
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
		err := setStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}

		// close phase1
		channels.quorum <- payload.Block
	}

	channels.mutex.Unlock()

	// TODO: cleanup memory store, datastore, etc. for each /block-number
}

func consumePhase2(payload PhasePayload, from peer.ID, datastore ds.Datastore, channels Channels) {
	validators, err := getValidatorsFromStore(payload, datastore)
	if err != nil {
		return
	}

	if includes := utils.IsValidator(from, validators); !includes {
		log.Printf("[WARN] Invalid validator: %s\n", from)
		return
	}

	channels.mutex.Lock()

	statusObj, err := getStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}

	phase2Completed := statusObj.PhaseCompleted >= 2

	if phase2Completed {
		// log.Printf("Phase 2 already completed. Status: %d\n", statusObj.PhaseCompleted)
		return
	}

	// `from` must be in committed set from phase1
	p1Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase1/%s", payload.Block, from))
	msgData, err := datastore.Get(context.Background(), p1Key)
	if err != nil {
		// log.Println(fmt.Sprintf("Error getting value from datastore for key: %s", p1Key), err)
		return
	}

	var p1Msg Phase1Data
	err = json.Unmarshal(msgData, &p1Msg)
	if err != nil {
		log.Printf("Error unmarshalling phase1 message: %v", err)
		return
	}

	if p1Msg.Delayed {
		return
	}

	data := PhaseData{Delayed: phase2Completed}
	p2Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// NOTE: currently, only valid {Delayed: false} (and already committed) phase2 messages are being stored
	p2Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	if err := datastore.Put(context.Background(), p2Key, p2Data); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", p2Key), err)
		return
	}

	// query data needed for deciding progress
	prefix := consensusPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, _, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	fmt.Println("<---- prefix:", len(validators), len(keys), prefix, from)

	// wait for 90th percentile response (among committed set from phase1)
	if len(keys)*10 >= len(validators)*9 {
		statusObj.PhaseCompleted = 2
		err := setStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}

		// close phase2
		channels.quorum <- payload.Block
	}

	channels.mutex.Unlock()
}

func consumePhase3(payload Phase3Payload, from peer.ID, datastore ds.Datastore, channels Channels) {
	_payload := PhasePayload{From: payload.From, Block: payload.Block, Phase: payload.Block}
	validators, err := getValidatorsFromStore(_payload, datastore)
	if err != nil {
		return
	}

	if includes := utils.IsValidator(from, validators); !includes {
		log.Printf("[WARN] Invalid validator: %s\n", from)
		return
	}

	// Entropy: [64 len hex] 32 bytes (256 bits)
	if len(payload.Entropy) != 64 {
		log.Printf("Invalid entropy: %s (%s)\n", payload.Entropy, from)
		return
	}

	channels.mutex.Lock()

	statusObj, err := getStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}

	phase3Completed := statusObj.PhaseCompleted >= 3

	// `from` must be in committed set from phase1
	p1Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase1/%s", payload.Block, from))
	msgData, err := datastore.Get(context.Background(), p1Key)
	if err != nil {
		// log.Println(fmt.Sprintf("Error getting value from datastore for key: %s", p1Key), err)
		return
	}

	var p1Msg Phase1Data
	err = json.Unmarshal(msgData, &p1Msg)
	if err != nil {
		log.Printf("Error unmarshalling phase1 message: %v", err)
		return
	}

	if p1Msg.Delayed {
		return
	}

	// verify reveal message
	hash := utils.CreateSHA3Hash(payload.Entropy)
	if hash != p1Msg.Hash {
		log.Printf("Unable to verify commitment: %s (%s)\n", p1Msg.Hash, payload.Entropy)
		return
	}

	data := Phase3Data{Entropy: payload.Entropy, Delayed: phase3Completed}
	p3Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	p3DataStr := string(p3Data)
	p3Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	if err := datastore.Put(context.Background(), p3Key, []byte(p3DataStr)); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", p3Key), err)
		return
	}

	if phase3Completed {
		return
	}

	// query data needed for deciding progress
	prefix := consensusPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, _, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	fmt.Println("<---- prefix:", len(validators), len(keys), prefix, from)

	// quorum: wait for 3/4 majority
	if len(keys)*4 >= len(validators)*3 {
		statusObj.PhaseCompleted = 3
		err := setStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}

		// close phase3
		channels.quorum <- payload.Block
	}

	channels.mutex.Unlock()
}

func listenConsensusTopic(node host.Host, topic *pubsub.Topic, datastore ds.Datastore) {
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

			var _payload PhasePayload
			err = json.Unmarshal([]byte(data), &_payload)
			if err != nil {
				log.Println("Error unmarshalling JSON:", err)
				return
			}

			if _payload.Phase == 1 {
				var payload Phase1Payload
				err = json.Unmarshal([]byte(data), &payload)
				if err != nil {
					log.Println("Error unmarshalling JSON:", err)
					return
				}

				// keep it outside goroutine for thread-safety of map
				key := fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
				channels := channelsFactory(key, channelsMap, func(block int) {
					triggerPhase2(block, node, topic)
				})

				go func() {
					consumePhase1(payload, from, datastore, channels)
				}()
			} else if _payload.Phase == 2 {
				// keep it outside goroutine for thread-safety of map
				key := fmt.Sprintf("/%d/phase%d", _payload.Block, _payload.Phase)
				channels := channelsFactory(key, channelsMap, func(block int) {
					triggerPhase3(block, node, topic, datastore)
				})

				go func() {
					consumePhase2(_payload, from, datastore, channels)
				}()
			} else if _payload.Phase == 3 {
				var payload Phase3Payload
				err = json.Unmarshal([]byte(data), &payload)
				if err != nil {
					log.Println("Error unmarshalling JSON:", err)
					return
				}

				// keep it outside goroutine for thread-safety of map
				key := fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
				channels := channelsFactory(key, channelsMap, func(block int) {
					triggerPhase4(block, node, topic)
				})

				go func() {
					consumePhase3(payload, from, datastore, channels)
				}()
			} else {
				log.Fatalf("[ERROR] Unhandled phase value: %d (%d)\n", _payload.Phase, _payload.Block)
			}
		}
	}(datastore)
}

func InitConsensusLoop(node host.Host, validators []peer.ID, ps *pubsub.PubSub, datastore ds.Datastore) {
	if includes := utils.IsValidator(node.ID(), validators); !includes {
		// current node is not a validator
		return
	}

	payloadValidator := func(ctx context.Context, pid peer.ID, msg *pubsub.Message) bool {
		// NOTE: `pid` could be an intermediary or relaying node, not necessarily the original publisher
		// unlike `msg.GetFrom()` which is the peer ID of the original publisher of the message
		return payloadDataValidator(msg)
	}
	err := ps.RegisterTopicValidator(consensusTopic, payloadValidator)
	if err != nil {
		panic(err)
	}
	topic, err := ps.Join(consensusTopic)
	if err != nil {
		panic(err)
	}
	listenConsensusTopic(node, topic, datastore)

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
				data := createPhase1Payload(node, nextBlockNumber, entropy)
				<-ticker
				go func() {
					triggerPhase1(entropy, nextBlockNumber, data, topic, validators, datastore)
				}()
			} else {
				time.Sleep(time.Duration(waitTimeMsec) * time.Millisecond)
			}
		}
	}()
}
