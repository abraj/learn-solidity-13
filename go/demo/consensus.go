package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"libp2pdemo/shared"
	"libp2pdemo/utils"
	"log"
	"math/big"
	"math/rand"
	"strconv"
	"sync"
	"time"

	vdf "github.com/abraj/vdf"
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
	From  string `json:"from"`
	Block int    `json:"block"`
	Phase int    `json:"phase"`
	Hash  string `json:"hash"`
	Salt  string `json:"salt"`
}

type Phase3Payload struct {
	From    string `json:"from"`
	Block   int    `json:"block"`
	Phase   int    `json:"phase"`
	Entropy string `json:"entropy"`
}

type Phase4Payload struct {
	From  string `json:"from"`
	Block int    `json:"block"`
	Phase int    `json:"phase"`
	View  string `json:"view"`
}

type Phase5Payload struct {
	From                 string `json:"from"`
	Block                int    `json:"block"`
	Phase                int    `json:"phase"`
	ValidatorsMerkleHash string `json:"validatorsMerkleHash"`
	Elapsed              int    `json:"elapsed"`
	View                 string `json:"view"`
}

type Phase6Payload struct {
	From    string `json:"from"`
	Block   int    `json:"block"`
	Phase   int    `json:"phase"`
	Seed    string `json:"seed"`    // accumulated entropy from contributions by validators
	Entropy string `json:"entropy"` // output from VDF delay function
	Delay   int64  `json:"delay"`   // delay in ms
	Steps   int64  `json:"steps"`   // number of steps for VDF delay function
}

type PhaseData struct {
	From string `json:"from"`
	// Delayed bool   `json:"delayed"`
}

type Phase1Data struct {
	From    string `json:"from"`
	Hash    string `json:"hash"`
	Salt    string `json:"salt"`
	Delayed bool   `json:"delayed"`
}

type Phase3Data struct {
	From    string `json:"from"`
	Entropy string `json:"entropy"`
	Delayed bool   `json:"delayed"`
}

type Phase4Data struct {
	From string `json:"from"`
	View string `json:"view"`
	// Delayed bool   `json:"delayed"`
}

type Phase5Data struct {
	From    string `json:"from"`
	IsValid bool   `json:"isValid"`
	Elapsed int    `json:"elapsed"`
	View    string `json:"view"`
}

type Phase6Data struct {
	From       string `json:"from"`
	Seed       string `json:"seed"`
	Entropy    string `json:"entropy"`
	Steps      int64  `json:"steps"`
	IsVerified bool   `json:"isVerified"`
}

type PhaseStatus struct {
	Entropy        string `json:"entropy"`
	PhaseCompleted int    `json:"phase"`
}

type Votes struct {
	Votes      int    `json:"votes"`
	VoteMap    string `json:"voteMap"`
	ElapsedMap string `json:"elapsedMap"`
}

type RandaoVotes struct {
	Votes        int               `json:"votes"`
	EntropyVotes map[string]int    `json:"entropyVotes"`
	EntropyMap   map[string]string `json:"entropyMap"`
}

type ConsensusObj struct {
	InFavor           []string `json:"inFavor"`
	Against           []string `json:"against"`
	Elapsed           int      `json:"elapsed"`
	InvalidValidators []string `json:"invalidValidators"`
	RandValue         string   `json:"randValue"`
}

type HashEntropy struct {
	hash    string
	salt    string
	entropy string
}

type Channels struct {
	mutex  *sync.Mutex
	quorum chan int
}

type Conds struct {
	cond *sync.Cond
}

const consensusTopic = "baadal-consensus"
const consensusPrefix = "/consensus/block"

func condsFactory(block int, phase int, condsMap map[string]Conds) Conds {
	key := fmt.Sprintf("/%d/phase%d", block, phase)
	if condsMap[key].cond == nil {
		var mu sync.Mutex
		cond := sync.NewCond(&mu)
		condsMap[key] = Conds{cond: cond}
	}
	return condsMap[key]
}

func channelsFactory(block int, phase int, channelsMap map[string]Channels, onQuorum func(block int)) Channels {
	key := fmt.Sprintf("/%d/phase%d", block, phase)
	if channelsMap[key].quorum == nil {
		timer := time.NewTimer(shared.PHASE_DURATION * time.Millisecond)

		// var mutex sync.Mutex
		mutex := sync.Mutex{}
		quorum := make(chan int)

		// mutex not needed (since no parallel goroutines) for `channelsFactory` execution
		channelsMap[key] = Channels{mutex: &mutex, quorum: quorum}

		go func() {
			defer timer.Stop()
			defer close(quorum)
			select {
			case block := <-quorum:
				if block == 0 {
					// impasse (no consensus)
					fmt.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>> impasse")
				} else {
					// quorum achieved
					go func() {
						onQuorum(block)
					}()
				}
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

	if !utils.Contains([]int{1, 2, 3, 4, 5, 6}, payload.Phase) {
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
	salt := utils.RandomHex(16)
	hash := utils.CreateSHA3Hash(entropy, salt)

	from := node.ID().String()
	payload := Phase1Payload{From: from, Block: blockNumber, Phase: 1, Hash: hash, Salt: salt}
	payloadData, _ := json.Marshal(payload)

	payloadStr := string(payloadData)
	return payloadStr
}

func createPhase2Payload(node host.Host, blockNumber int) string {
	from := node.ID().String()
	payload := PhasePayload{From: from, Block: blockNumber, Phase: 2}
	payloadData, _ := json.Marshal(payload)

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
	payloadData, _ := json.Marshal(payload)

	payloadStr := string(payloadData)
	return payloadStr
}

func createPhase4Payload(node host.Host, blockNumber int, datastore ds.Datastore) string {
	prefix := consensusPrefix + fmt.Sprintf("/%d/phase3", blockNumber)
	_, values, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return ""
	}

	viewItems := []string{}
	for _, value := range values {
		var p3Obj Phase3Data
		err := json.Unmarshal([]byte(value), &p3Obj)
		if err != nil {
			log.Println("Error unmarshalling value JSON:", err)
			return ""
		}
		if !p3Obj.Delayed {
			viewItems = append(viewItems, p3Obj.From)
		}
	}

	view, err := json.Marshal(viewItems)
	if err != nil {
		log.Println("Error marshalling view list:", err)
		return ""
	}

	from := node.ID().String()
	payload := Phase4Payload{From: from, Block: blockNumber, Phase: 4, View: string(view)}
	payloadData, _ := json.Marshal(payload)

	payloadStr := string(payloadData)
	return payloadStr
}

func createPhase5Payload(node host.Host, blockNumber int, datastore ds.Datastore) string {
	validators, err := getValidatorsFromStore(blockNumber, datastore)
	if err != nil {
		return ""
	}

	voteMap := make(map[string]int, len(validators))
	partisanMap := make(map[string]bool, len(validators))
	for _, validator := range validators {
		voteMap[validator.String()] = 0
		partisanMap[validator.String()] = false
	}

	partisanMap[node.ID().String()] = true

	// handle tally for current node
	{
		prefix := consensusPrefix + fmt.Sprintf("/%d/phase3", blockNumber)
		_, values, err := QueryWithPrefix(datastore, prefix)
		if err != nil {
			log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
			return ""
		}

		viewItems := []string{}
		for _, value := range values {
			var p3Obj Phase3Data
			err := json.Unmarshal([]byte(value), &p3Obj)
			if err != nil {
				log.Println("Error unmarshalling value JSON:", err)
				return ""
			}
			if !p3Obj.Delayed {
				viewItems = append(viewItems, p3Obj.From)
			}
		}

		var mu sync.Mutex
		for _, item := range viewItems {
			mu.Lock()
			value, ok := partisanMap[item]
			if ok {
				if !value {
					partisanMap[item] = true
				}
			} else {
				fmt.Printf("[WARN] item not in validator list: %s\n", item)
			}
			mu.Unlock()
		}
	}

	// aggregate votes: handle tally from incoming messages
	{
		prefix := consensusPrefix + fmt.Sprintf("/%d/phase4", blockNumber)
		_, values, err := QueryWithPrefix(datastore, prefix)
		if err != nil {
			log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
			return ""
		}

		for _, value := range values {
			var p4Obj Phase4Data
			err := json.Unmarshal([]byte(value), &p4Obj)
			if err != nil {
				log.Println("Error unmarshalling value JSON:", err)
				continue
			}

			viewStr := p4Obj.View
			viewItems := make([]string, 0)
			err = json.Unmarshal([]byte(viewStr), &viewItems)
			if err != nil {
				fmt.Printf("Unable to unmarshall view: %v\n", err)
				continue
			}

			var mu sync.Mutex
			for _, item := range viewItems {
				mu.Lock()
				value, ok := voteMap[item]
				if ok {
					voteMap[item] = value + 1
				} else {
					fmt.Printf("[WARN] item not in validator list: %s\n", item)
				}
				mu.Unlock()
			}
		}
	}

	var mu sync.Mutex
	for _, validator := range validators {
		item := validator.String()
		if partisanMap[item] {
			continue
		}

		value, ok := voteMap[item]
		// simple (50%) majority for accepting suggestions from other nodes
		if ok && value*2 >= len(validators) {
			mu.Lock()
			curr, ok2 := partisanMap[item]
			if ok2 && !curr {
				// NOTE: (entropy/commitment for) these items (suggested by other nodes)
				// are not verified as of now.
				// Needs to be verified (locally) after phase5 response
				partisanMap[item] = true
			}
			mu.Unlock()
		}
	}

	viewItems := []string{}
	for key, value := range partisanMap {
		if value {
			viewItems = append(viewItems, key)
		}
	}

	view, err := json.Marshal(viewItems)
	if err != nil {
		log.Println("Error marshalling view list:", err)
		return ""
	}

	_, timeLeftMsec := shared.SlotInfo()
	timeElapsedMsec := shared.SLOT_DURATION - timeLeftMsec

	validatorsList := utils.Map(validators, func(v peer.ID) string {
		return v.String()
	})
	validatorsMerkleHash := utils.MerkleHash(validatorsList)

	from := node.ID().String()
	payload := Phase5Payload{From: from, Block: blockNumber, Phase: 5, ValidatorsMerkleHash: validatorsMerkleHash, Elapsed: timeElapsedMsec, View: string(view)}
	payloadData, _ := json.Marshal(payload)

	payloadStr := string(payloadData)
	return payloadStr
}

func createPhase6Payload(node host.Host, blockNumber int, datastore ds.Datastore) string {
	validators, err := getValidatorsFromStore(blockNumber, datastore)
	if err != nil {
		return ""
	}

	hashMap := make(map[string]HashEntropy)
	for _, validator := range validators {
		item := validator.String()
		hashMap[item] = HashEntropy{}
	}

	{
		prefix := consensusPrefix + fmt.Sprintf("/%d/phase1", blockNumber)
		_, values, err := QueryWithPrefix(datastore, prefix)
		if err != nil {
			log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
			return ""
		}
		for _, value := range values {
			var p1Obj Phase1Data
			err := json.Unmarshal([]byte(value), &p1Obj)
			if err != nil {
				log.Printf("Unable to unmarshal: %s %v\n", value, err)
				continue
			}
			item := p1Obj.From
			tuple, ok := hashMap[item]
			if !ok {
				log.Printf("Unexpected phase1 key: %s\n", item)
				continue
			}
			tuple.hash = p1Obj.Hash
			tuple.salt = p1Obj.Salt
			hashMap[item] = tuple
		}
	}

	{
		prefix := consensusPrefix + fmt.Sprintf("/%d/phase3", blockNumber)
		_, values, err := QueryWithPrefix(datastore, prefix)
		if err != nil {
			log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
			return ""
		}
		for _, value := range values {
			var p3Obj Phase3Data
			err := json.Unmarshal([]byte(value), &p3Obj)
			if err != nil {
				log.Printf("Unable to unmarshal: %s %v\n", value, err)
				continue
			}
			item := p3Obj.From
			tuple, ok := hashMap[item]
			if !ok {
				log.Printf("Unexpected phase3 key: %s\n", item)
				continue
			}
			tuple.entropy = p3Obj.Entropy
			hashMap[item] = tuple
		}
	}

	consensusKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/consensus", blockNumber))
	consensusData, err := datastore.Get(context.Background(), consensusKey)
	if err != nil {
		log.Println(fmt.Sprintf("Error getting data for key: %s", consensusKey), err)
		return ""
	}

	var consensusObj ConsensusObj
	err = json.Unmarshal(consensusData, &consensusObj)
	if err != nil {
		log.Printf("Unable to unmarshal consensus data: %s %v\n", consensusData, err)
		return ""
	}

	if len(hashMap) != len(consensusObj.InFavor)+len(consensusObj.Against) {
		log.Printf("Consensus tally mismatch: %d (%d, %d)\n", len(hashMap), len(consensusObj.InFavor), len(consensusObj.Against))
		return ""
	}

	// verify all commit reveals again since some of the items might be (majority) suggested by others
	// Assumption: Messages corresponding to all peerIDs in the consensus favor list
	//  have arrived (as part of the phase1 and phase3 responses) to the current node
	for _, item := range consensusObj.InFavor {
		value := hashMap[item]
		if value.hash == "" || value.entropy == "" || value.salt == "" {
			log.Printf("entropy/hash does not exist in hashMap for key: %s (%s, %s, %s)\n", item, value.entropy, value.hash, value.salt)
			return ""
		}

		// verify commit reveal (again)
		hash := utils.CreateSHA3Hash(value.entropy, value.salt)
		if hash == "" || hash != value.hash {
			log.Printf("Unable to verify commitment for key: %s (%s, %s, %s)\n", item, value.entropy, value.hash, value.salt)
			return ""
		}
	}

	consensusList := make([]string, len(consensusObj.InFavor))
	copy(consensusList, consensusObj.InFavor)

	composedHash := ""
	for _, item := range consensusList {
		composedHash = utils.CreateSHA3Hash(composedHash+hashMap[item].entropy, "")
	}

	// VDF delay
	elapsedMsec := utils.MaxInt(consensusObj.Elapsed, 200)
	t := utils.MsecToSteps(elapsedMsec)
	x, _ := new(big.Int).SetString(composedHash, 16)
	v := vdf.NewVDF()
	t1 := time.Now()
	y := v.Delay(t, x) // VDF delay
	t2 := time.Now()

	if y == nil {
		log.Printf("[VDF] Unable to create VDF output! Possibly unsupported prime (p ≢ 3 mod 4) used. x: %v y: %v\n", x, y)
		log.Printf("[VDF] Prime used: %v\n", v.GetModulus())
		return ""
	} else {
		if !v.Verify(t, x, y) {
			log.Printf("[VDF] Unable to verify VDF output! Input (seed) to VDF possibly too large. x: %v y: %v\n", x, y)
			log.Printf("[VDF] Prime used: %v\n", v.GetModulus())
			return ""
		}
	}

	entropy := y.Text(16)
	delay := t2.Sub(t1).Milliseconds()

	from := node.ID().String()
	payload := Phase6Payload{From: from, Block: blockNumber, Phase: 6, Seed: composedHash, Entropy: entropy, Delay: delay, Steps: t}
	payloadData, _ := json.Marshal(payload)

	payloadStr := string(payloadData)
	return payloadStr
}

func initBlockData(blockNumber int, datastore ds.Datastore) []peer.ID {
	validators := GetValidatorsList()
	validatorsList, err := json.Marshal(validators)
	if err != nil {
		log.Println("Error marshalling validators list:", err)
		return nil
	}
	validatorsListStr := string(validatorsList)

	validatorsKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/validators", blockNumber))
	if err := datastore.Put(context.Background(), validatorsKey, []byte(validatorsListStr)); err != nil {
		log.Println(fmt.Sprintf("Error setting data for key: %s", validatorsKey), err)
		return nil
	}

	votesObjStr, _ := json.Marshal(Votes{Votes: 0, VoteMap: "{}", ElapsedMap: "{}"})
	votesKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/votes", blockNumber))
	if err := datastore.Put(context.Background(), votesKey, []byte(votesObjStr)); err != nil {
		log.Println(fmt.Sprintf("Error setting data for key: %s", votesKey), err)
		return nil
	}

	rvotesObjStr, _ := json.Marshal(RandaoVotes{Votes: 0, EntropyVotes: map[string]int{}, EntropyMap: map[string]string{}})
	rvotesKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/randao-votes", blockNumber))
	if err := datastore.Put(context.Background(), rvotesKey, []byte(rvotesObjStr)); err != nil {
		log.Println(fmt.Sprintf("Error setting data for key: %s", rvotesKey), err)
		return nil
	}

	return validators
}

// phase1: commit phase
func triggerPhase1(entropy string, nextBlockNumber int, data string, topic *pubsub.Topic, datastore ds.Datastore) {
	shared.SetLatestBlockNumber(nextBlockNumber)

	// var payload Phase1Payload
	// err := json.Unmarshal([]byte(data), &payload)
	// if err != nil {
	// 	log.Println("Error unmarshalling JSON:", err)
	// 	return
	// }

	statusObj, err := getStatusObj(nextBlockNumber, datastore)
	if err != nil {
		return
	}

	statusObj.Entropy = entropy
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

// phase4: partial-tally phase
func triggerPhase4(block int, node host.Host, topic *pubsub.Topic, datastore ds.Datastore) {
	fmt.Println("triggerPhase4..", block)

	data := createPhase4Payload(node, block, datastore)
	if data == "" {
		return
	}

	err := topic.Publish(context.Background(), []byte(data))
	if err != nil {
		log.Printf("Could not publish invalid message: %v\n", err)
		// panic(err)
	}
}

// phase5: full-tally phase
func triggerPhase5(block int, node host.Host, topic *pubsub.Topic, datastore ds.Datastore) {
	fmt.Println("triggerPhase5..", block)

	data := createPhase5Payload(node, block, datastore)
	if data == "" {
		return
	}

	err := topic.Publish(context.Background(), []byte(data))
	if err != nil {
		log.Printf("Could not publish invalid message: %v\n", err)
		// panic(err)
	}
}

// phase6: VDF delay phase
func triggerPhase6(block int, node host.Host, topic *pubsub.Topic, datastore ds.Datastore) {
	fmt.Println("triggerPhase6..", block)

	data := createPhase6Payload(node, block, datastore)
	if data == "" {
		return
	}

	err := topic.Publish(context.Background(), []byte(data))
	if err != nil {
		log.Printf("Could not publish invalid message: %v\n", err)
		// panic(err)
	}
}

func randaoConsensus(block int, datastore ds.Datastore) {
	fmt.Println("randaoConsensus..", block)

	consensusKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/consensus", block))
	consensusData, _ := datastore.Get(context.Background(), consensusKey)

	var consensusObj ConsensusObj
	json.Unmarshal(consensusData, &consensusObj)

	randaoMix := consensusObj.RandValue
	// fmt.Println("InFavor:", consensusObj.InFavor)
	fmt.Println("randaoMix:", randaoMix)
}

func getStatusObj(block int, datastore ds.Datastore) (PhaseStatus, error) {
	statusKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/status", block))
	statusData, err := datastore.Get(context.Background(), statusKey)
	if err != nil {
		statusObj := PhaseStatus{PhaseCompleted: 0}
		if err := setStatusObj(statusObj, block, datastore); err == nil {
			return statusObj, nil
		}
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

func getValidatorsFromStore(block int, datastore ds.Datastore) ([]peer.ID, error) {
	validatorsKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/validators", block))
	validatorsList, err := datastore.Get(context.Background(), validatorsKey)
	if err != nil {
		validators := initBlockData(block, datastore)
		if len(validators) > 0 {
			return validators, nil
		}
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

func consumePhase1(node host.Host, payload Phase1Payload, from peer.ID, datastore ds.Datastore, channels Channels, conds Conds) {
	validators, err := getValidatorsFromStore(payload.Block, datastore)
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

	// Salt: [32 len hex] 16 bytes
	if len(payload.Salt) != 32 {
		log.Printf("Invalid salt: %s (%s)\n", payload.Salt, from)
		return
	}

	channels.mutex.Lock()
	defer channels.mutex.Unlock()

	statusObj, err := getStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}

	phase1Completed := statusObj.PhaseCompleted >= 1

	p1Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	_, err = datastore.Get(context.Background(), p1Key)
	if err == nil {
		log.Printf("Value already exists in datastore for key: %s", p1Key)
		return
	}

	data := Phase1Data{From: payload.From, Hash: payload.Hash, Salt: payload.Salt, Delayed: phase1Completed}
	p1Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// move below to avoid capturing laggard/delayed messages
	p1DataStr := string(p1Data)
	// p1Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
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

	allKeys := utils.Map(keys, func(key ds.Key) string {
		return key.String()
	})
	hostKey := consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, node.ID().String())
	selfAccountedFor := utils.Contains(allKeys, hostKey)

	fmt.Println("<---- prefix:", len(validators), len(keys), prefix, from)
	// fmt.Println("keys:", keys)
	// fmt.Println("values:", values)

	// wait for 90th percentile response
	if selfAccountedFor && len(keys)*10 >= len(validators)*9 {
		statusObj.PhaseCompleted = 1
		err := setStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}
		conds.cond.Broadcast()

		// close phase1
		channels.quorum <- payload.Block
	}

	// TODO: cleanup memory store, datastore, etc. for each /block-number
}

func consumePhase2(node host.Host, payload PhasePayload, from peer.ID, datastore ds.Datastore, channels Channels, conds Conds, condsPrev Conds) {
	validators, err := getValidatorsFromStore(payload.Block, datastore)
	if err != nil {
		return
	}

	if includes := utils.IsValidator(from, validators); !includes {
		log.Printf("[WARN] Invalid validator: %s\n", from)
		return
	}

	channels.mutex.Lock()
	defer channels.mutex.Unlock()

	statusObj, err := getStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}

	if statusObj.PhaseCompleted < 1 {
		condsPrev.cond.L.Lock()
		condsPrev.cond.Wait()
		condsPrev.cond.L.Unlock()
	}

	phase2Completed := statusObj.PhaseCompleted >= 2

	if phase2Completed {
		// log.Printf("Phase 2 already completed. Status: %d\n", statusObj.PhaseCompleted)
		return
	}

	p2Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	_, err = datastore.Get(context.Background(), p2Key)
	if err == nil {
		log.Printf("Value already exists in datastore for key: %s", p2Key)
		return
	}

	// `from` must be in committed set from phase1
	p1Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase1/%s", payload.Block, from))
	p1MsgData, err := datastore.Get(context.Background(), p1Key)
	if err != nil {
		// log.Println(fmt.Sprintf("Error getting value from datastore for key: %s", p1Key), err)
		return
	}

	var p1Msg Phase1Data
	err = json.Unmarshal(p1MsgData, &p1Msg)
	if err != nil {
		log.Printf("Error unmarshalling phase1 message: %v", err)
		return
	}

	// only consider if part of phase1 cut-off
	if p1Msg.Delayed {
		return
	}

	data := PhaseData{From: payload.From /*, Delayed: phase2Completed*/}
	p2Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// NOTE: currently, only valid (part of phase1 cut-off and already committed) phase2 messages are being stored
	// Also, stop storing messages as soon as (phase2) cut-off is reached
	// p2Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
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

	allKeys := utils.Map(keys, func(key ds.Key) string {
		return key.String()
	})
	hostKey := consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, node.ID().String())
	selfAccountedFor := utils.Contains(allKeys, hostKey)

	fmt.Println("<---- prefix:", len(validators), len(keys), prefix, from)

	// wait for 90th percentile response (among committed set from phase1)
	if selfAccountedFor && len(keys)*100 >= len(validators)*81 {
		statusObj.PhaseCompleted = 2
		err := setStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}
		conds.cond.Broadcast()

		// close phase2
		channels.quorum <- payload.Block
	}
}

func consumePhase3(node host.Host, payload Phase3Payload, from peer.ID, datastore ds.Datastore, channels Channels, conds Conds, condsPrev Conds) {
	validators, err := getValidatorsFromStore(payload.Block, datastore)
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
	defer channels.mutex.Unlock()

	statusObj, err := getStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}

	if statusObj.PhaseCompleted < 2 {
		condsPrev.cond.L.Lock()
		condsPrev.cond.Wait()
		condsPrev.cond.L.Unlock()
	}

	phase3Completed := statusObj.PhaseCompleted >= 3

	p3Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	_, err = datastore.Get(context.Background(), p3Key)
	if err == nil {
		log.Printf("Value already exists in datastore for key: %s", p3Key)
		return
	}

	p1Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase1/%s", payload.Block, from))
	p1MsgData, err := datastore.Get(context.Background(), p1Key)
	if err != nil {
		// log.Println(fmt.Sprintf("Error getting value from datastore for key: %s", p1Key), err)
		return
	}

	var p1Msg Phase1Data
	err = json.Unmarshal(p1MsgData, &p1Msg)
	if err != nil {
		log.Printf("Error unmarshalling phase1 message: %v", err)
		return
	}

	// verify commit reveal
	hash := utils.CreateSHA3Hash(payload.Entropy, p1Msg.Salt)
	if hash == "" || hash != p1Msg.Hash {
		log.Printf("Unable to verify commitment: %s (%s)\n", p1Msg.Hash, payload.Entropy)
		return
	}

	data := Phase3Data{From: payload.From, Entropy: payload.Entropy, Delayed: p1Msg.Delayed}
	p3Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// NOTE: keep storing phase3 messages even after 3/4 majority
	// Assumption: phase3 message for a node (for a block number) arrives after phase1 message
	// NOTE: phase3 message for a node is considered delayed if the corresponding phase1 message is delayed
	p3DataStr := string(p3Data)
	// p3Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	if err := datastore.Put(context.Background(), p3Key, []byte(p3DataStr)); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", p3Key), err)
		return
	}

	if phase3Completed {
		// log.Printf("Phase 3 already completed. Status: %d\n", statusObj.PhaseCompleted)
		return
	}

	// query data needed for deciding progress
	prefix := consensusPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, _, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	allKeys := utils.Map(keys, func(key ds.Key) string {
		return key.String()
	})
	hostKey := consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, node.ID().String())
	selfAccountedFor := utils.Contains(allKeys, hostKey)

	fmt.Println("<---- prefix:", len(validators), len(keys), prefix, from)

	// wait for 3/4 responses
	if selfAccountedFor && len(keys)*4 >= len(validators)*3 {
		statusObj.PhaseCompleted = 3
		err := setStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}
		conds.cond.Broadcast()

		// close phase3
		channels.quorum <- payload.Block
	}
}

func consumePhase4(node host.Host, payload Phase4Payload, from peer.ID, datastore ds.Datastore, channels Channels, conds Conds, condsPrev Conds) {
	validators, err := getValidatorsFromStore(payload.Block, datastore)
	if err != nil {
		return
	}

	if includes := utils.IsValidator(from, validators); !includes {
		log.Printf("[WARN] Invalid validator: %s\n", from)
		return
	}

	channels.mutex.Lock()
	defer channels.mutex.Unlock()

	statusObj, err := getStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}

	if statusObj.PhaseCompleted < 3 {
		condsPrev.cond.L.Lock()
		condsPrev.cond.Wait()
		condsPrev.cond.L.Unlock()
	}

	phase4Completed := statusObj.PhaseCompleted >= 4

	p4Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	_, err = datastore.Get(context.Background(), p4Key)
	if err == nil {
		log.Printf("Value already exists in datastore for key: %s", p4Key)
		return
	}

	data := Phase4Data{From: payload.From, View: payload.View /*, Delayed: phase4Completed*/}
	p4Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// NOTE: keep storing phase4 messages
	p4DataStr := string(p4Data)
	// p4Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	if err := datastore.Put(context.Background(), p4Key, []byte(p4DataStr)); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", p4Key), err)
		return
	}

	if phase4Completed {
		// log.Printf("Phase 4 already completed. Status: %d\n", statusObj.PhaseCompleted)
		return
	}

	// query data needed for deciding progress
	prefix := consensusPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, _, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	allKeys := utils.Map(keys, func(key ds.Key) string {
		return key.String()
	})
	hostKey := consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, node.ID().String())
	selfAccountedFor := utils.Contains(allKeys, hostKey)

	fmt.Println("<---- prefix:", len(validators), len(keys), prefix, from)

	// wait for 90th percentile response
	if selfAccountedFor && len(keys)*10 >= len(validators)*9 {
		statusObj.PhaseCompleted = 4
		err := setStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}
		conds.cond.Broadcast()

		// close phase4
		channels.quorum <- payload.Block
	}
}

func consumePhase5(node host.Host, payload Phase5Payload, from peer.ID, datastore ds.Datastore, channels Channels, conds Conds, condsPrev Conds) {
	validators, err := getValidatorsFromStore(payload.Block, datastore)
	if err != nil {
		return
	}

	if includes := utils.IsValidator(from, validators); !includes {
		log.Printf("[WARN] Invalid validator: %s\n", from)
		return
	}

	// match validator list
	validatorsList := utils.Map(validators, func(v peer.ID) string {
		return v.String()
	})
	validatorsMerkleHash := utils.MerkleHash(validatorsList)

	// NOTE: do not return here, even in case of validator list "mismatch"
	// as the mismatch needs to be saved
	isValidatorsMatch := payload.ValidatorsMerkleHash == validatorsMerkleHash

	channels.mutex.Lock()
	defer channels.mutex.Unlock()

	statusObj, err := getStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}

	if statusObj.PhaseCompleted < 4 {
		condsPrev.cond.L.Lock()
		condsPrev.cond.Wait()
		condsPrev.cond.L.Unlock()
	}

	phase5Completed := statusObj.PhaseCompleted >= 5

	if phase5Completed {
		// log.Printf("Phase 5 already completed. Status: %d\n", statusObj.PhaseCompleted)
		return
	}

	p5Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	_, err = datastore.Get(context.Background(), p5Key)
	if err == nil {
		log.Printf("Value already exists in datastore for key: %s", p5Key)
		return
	}

	data := Phase5Data{From: payload.From, IsValid: isValidatorsMatch, Elapsed: payload.Elapsed, View: payload.View}
	p5Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// store phase5 response messages
	p5DataStr := string(p5Data)
	// p5Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	if err := datastore.Put(context.Background(), p5Key, []byte(p5DataStr)); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", p5Key), err)
		return
	}

	// NOTE: placed after the above DB save operation since we need to store
	// (potentially invalid) response messages
	if !isValidatorsMatch {
		log.Printf("Validators list mismatch: %s %s\n", payload.ValidatorsMerkleHash, validatorsMerkleHash)
		return
	}

	// query saved response messages
	prefix := consensusPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, values, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	invalidValidators := []string{}
	for _, value := range values {
		var p5Data Phase5Data
		json.Unmarshal([]byte(value), &p5Data)
		if !p5Data.IsValid {
			invalidValidators = append(invalidValidators, p5Data.From)
		}
	}

	if len(invalidValidators)*3 > len(validators) {
		log.Printf("Too many invalid validators: %d (%d)\n", len(invalidValidators), len(validators))
		return
	}
	validValidatorsLen := len(validators) - len(invalidValidators)

	votesKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/votes", payload.Block))
	votesData, err := datastore.Get(context.Background(), votesKey)
	if err != nil {
		log.Println(fmt.Sprintf("Error getting data for key: %s", votesKey), err)
		return
	}

	var votesObj Votes
	err = json.Unmarshal(votesData, &votesObj)
	if err != nil {
		log.Printf("[ERROR] Unable to unmarshal votes data: %s\n", string(votesData))
		return
	}

	voteMap := make(map[string]int)
	err = json.Unmarshal([]byte(votesObj.VoteMap), &voteMap)
	if err != nil {
		log.Printf("[ERROR] Unable to unmarshal voteMap data: %s\n", votesObj.VoteMap)
		return
	}

	elapsedMap := make(map[string]int)
	err = json.Unmarshal([]byte(votesObj.ElapsedMap), &elapsedMap)
	if err != nil {
		log.Printf("[ERROR] Unable to unmarshal elapsedMap data: %s\n", votesObj.ElapsedMap)
		return
	}

	view := make([]string, 0, len(validators))
	err = json.Unmarshal([]byte(data.View), &view)
	if err != nil {
		fmt.Printf("[WARN] Invalid view data: %s\n", data.View)
		return
	}
	voteItemMap := make(map[string]bool)
	for _, item := range view {
		voteItemMap[item] = true
	}

	votesObj.Votes += 1
	elapsedMap[from.String()] = payload.Elapsed
	for _, validator := range validators {
		item := validator.String()
		if utils.Contains(invalidValidators, item) {
			continue
		}
		if voteItemMap[item] {
			value, ok := voteMap[item]
			if ok {
				voteMap[item] = value + 1
			} else {
				voteMap[item] = 1
			}
		}
	}

	fmt.Println("<---- data:", len(validators), votesObj.Votes, "phase5", data.From)

	voteMapData, _ := json.Marshal(voteMap)
	elapsedMapData, _ := json.Marshal(elapsedMap)
	votesObj.VoteMap = string(voteMapData)
	votesObj.ElapsedMap = string(elapsedMapData)

	votesObjStr, err := json.Marshal(votesObj)
	if err != nil {
		fmt.Printf("[WARN] Unable to marshal votesObj: %v\n", votesObj)
		return
	}

	// votesKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/votes", payload.Block))
	if err := datastore.Put(context.Background(), votesKey, []byte(votesObjStr)); err != nil {
		log.Println(fmt.Sprintf("Error setting data for key: %s", votesKey), err)
		return
	}

	total := votesObj.Votes
	if total*3 < validValidatorsLen*2 {
		// not enough votes to reach consensus
		return
	}

	// test for consensus
	consensus := true
	inFavorList := []string{}
	againstList := []string{}
	elapsedList := []int{}
	for _, validator := range validators {
		item := validator.String()
		if utils.Contains(invalidValidators, item) {
			continue
		}
		inFavor := voteMap[item]
		against := total - inFavor
		if inFavor*3 >= validValidatorsLen*2 {
			// consensus: `yes` for item
			inFavorList = append(inFavorList, item)
			elapsedList = append(elapsedList, elapsedMap[item])
			continue
		} else if against*3 >= validValidatorsLen*2 {
			// consensus: `no` for item
			againstList = append(againstList, item)
			continue
		}
		consensus = false
		break
	}

	retVal := -1
	if consensus {
		// consensus
		retVal = payload.Block

		// save consensus data
		elapsed := utils.Median(elapsedList)
		consensusObj := ConsensusObj{InFavor: inFavorList, Against: againstList, Elapsed: elapsed, InvalidValidators: invalidValidators}
		consensusObjData, _ := json.Marshal(consensusObj)
		consensusKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/consensus", payload.Block))
		err := datastore.Put(context.Background(), consensusKey, consensusObjData)
		if err != nil {
			log.Println(fmt.Sprintf("Error putting data for key: %s", consensusKey), err)
			return
		}
	} else if total >= validValidatorsLen {
		// impasse (no consensus)
		retVal = 0
	}

	allKeys := utils.Map(keys, func(key ds.Key) string {
		return key.String()
	})
	hostKey := consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, node.ID().String())
	selfAccountedFor := utils.Contains(allKeys, hostKey)

	if selfAccountedFor && retVal >= 0 {
		statusObj.PhaseCompleted = 5
		err := setStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}
		conds.cond.Broadcast()

		// close phase5
		channels.quorum <- retVal
	}
}

func consumePhase6(node host.Host, payload Phase6Payload, from peer.ID, datastore ds.Datastore, channels Channels, conds Conds, condsPrev Conds) {
	validators, err := getValidatorsFromStore(payload.Block, datastore)
	if err != nil {
		return
	}

	if includes := utils.IsValidator(from, validators); !includes {
		log.Printf("[WARN] Invalid validator: %s\n", from)
		return
	}

	consensusKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/consensus", payload.Block))
	consensusData, err := datastore.Get(context.Background(), consensusKey)
	if err != nil {
		log.Println(fmt.Sprintf("Error getting data for key: %s", consensusKey), err)
		return
	}

	var consensusObj ConsensusObj
	err = json.Unmarshal(consensusData, &consensusObj)
	if err != nil {
		log.Printf("Unable to unmarshal consensus data: %s %v\n", consensusData, err)
		return
	}

	invalidValidators := consensusObj.InvalidValidators
	validValidatorsLen := len(validators) - len(invalidValidators)

	if utils.Contains(invalidValidators, from.String()) {
		log.Printf("[WARN] Invalid validators list at this validator: %s\n", from)
		return
	}

	channels.mutex.Lock()
	defer channels.mutex.Unlock()

	statusObj, err := getStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}

	if statusObj.PhaseCompleted < 5 {
		condsPrev.cond.L.Lock()
		condsPrev.cond.Wait()
		condsPrev.cond.L.Unlock()
	}

	phase6Completed := statusObj.PhaseCompleted >= 6

	if phase6Completed {
		// log.Printf("Phase 6 already completed. Status: %d\n", statusObj.PhaseCompleted)
		return
	}

	p6Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	_, err = datastore.Get(context.Background(), p6Key)
	if err == nil {
		log.Printf("Value already exists in datastore for key: %s", p6Key)
		return
	}

	// VDF verify
	t := payload.Steps
	x, _ := new(big.Int).SetString(payload.Seed, 16)
	y, _ := new(big.Int).SetString(payload.Entropy, 16)
	v := vdf.NewVDF()
	isVerified := v.Verify(t, x, y)

	data := Phase6Data{From: payload.From, Seed: payload.Seed, Entropy: payload.Entropy, Steps: payload.Steps, IsVerified: isVerified}
	p6Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// store phase6 response messages
	p6DataStr := string(p6Data)
	// p6Key := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	if err := datastore.Put(context.Background(), p6Key, []byte(p6DataStr)); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", p6Key), err)
		return
	}

	// query saved response messages
	prefix := consensusPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, _, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	rvotesKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/randao-votes", payload.Block))
	rvotesData, err := datastore.Get(context.Background(), rvotesKey)
	if err != nil {
		log.Println(fmt.Sprintf("Error getting data for key: %s", rvotesKey), err)
		return
	}

	var rvotesObj RandaoVotes
	err = json.Unmarshal(rvotesData, &rvotesObj)
	if err != nil {
		log.Printf("[ERROR] Unable to unmarshal randao-votes data: %s\n", string(rvotesData))
		return
	}

	rvotesObj.Votes += 1

	fmt.Println("<---- data:", len(validators), rvotesObj.Votes, "phase6", data.From)

	if !isVerified {
		log.Printf("[ERROR] VDF verification failed! x: %s y: %s t: %d\n", payload.Seed, payload.Entropy, payload.Steps)
		return
	}

	rKey := utils.CreateSHA3Hash(payload.Seed+payload.Entropy+strconv.FormatInt(payload.Steps, 10), "")
	eVotes, ok := rvotesObj.EntropyVotes[rKey]
	if ok {
		rvotesObj.EntropyVotes[rKey] = eVotes + 1
	} else {
		rvotesObj.EntropyVotes[rKey] = 1
		rvotesObj.EntropyMap[rKey] = payload.Entropy
	}

	rvotesObjStr, err := json.Marshal(rvotesObj)
	if err != nil {
		fmt.Printf("[WARN] Unable to marshal votesObj: %v\n", rvotesObj)
		return
	}

	// rvotesKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/randao-votes", payload.Block))
	if err := datastore.Put(context.Background(), rvotesKey, []byte(rvotesObjStr)); err != nil {
		log.Println(fmt.Sprintf("Error setting data for key: %s", rvotesKey), err)
		return
	}

	total := rvotesObj.Votes
	if total*3 < validValidatorsLen*2 {
		// not enough votes to reach consensus
		return
	}

	// test for consensus
	consensus := false
	var consensusRandValue string
	for key, value := range rvotesObj.EntropyVotes {
		if value*3 >= validValidatorsLen*2 {
			consensus = true
			consensusRandValue = rvotesObj.EntropyMap[key]
			break
		}
	}

	retVal := -1
	if consensus {
		// consensus
		retVal = payload.Block

		// save consensus data
		consensusObj.RandValue = consensusRandValue
		consensusObjData, _ := json.Marshal(consensusObj)
		consensusKey := ds.NewKey(consensusPrefix + fmt.Sprintf("/%d/consensus", payload.Block))
		err := datastore.Put(context.Background(), consensusKey, consensusObjData)
		if err != nil {
			log.Println(fmt.Sprintf("Error putting data for key: %s", consensusKey), err)
			return
		}
	} else if total >= validValidatorsLen {
		// impasse (no consensus)
		retVal = 0
	}

	allKeys := utils.Map(keys, func(key ds.Key) string {
		return key.String()
	})
	hostKey := consensusPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, node.ID().String())
	selfAccountedFor := utils.Contains(allKeys, hostKey)

	if selfAccountedFor && retVal >= 0 {
		statusObj.PhaseCompleted = 6
		err := setStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}
		conds.cond.Broadcast()

		// close phase6
		channels.quorum <- retVal
	}
}

func listenConsensusTopic(node host.Host, topic *pubsub.Topic, datastore ds.Datastore) {
	subs, err := topic.Subscribe()
	if err != nil {
		panic(err)
	}

	go func(datastore ds.Datastore) {
		defer subs.Cancel()

		channelsMap := make(map[string]Channels)
		condsMap := make(map[string]Conds)
		for {
			msg, err := subs.Next(context.Background())
			if err != nil {
				log.Println("Error receiving message:", err)
				continue
			}

			data := string(msg.Data)
			from := msg.GetFrom()

			var _payload PhasePayload
			err = json.Unmarshal([]byte(data), &_payload)
			if err != nil {
				log.Println("Error unmarshalling JSON:", err)
				continue
			}

			statusObj, err := getStatusObj(_payload.Block, datastore)
			if err != nil {
				log.Println("Unable to get statusObj:", err)
				continue
			}
			ongoingPhase := statusObj.PhaseCompleted + 1
			maxAllowedPhase := ongoingPhase + 1

			if _payload.Phase > maxAllowedPhase {
				log.Printf("[WARN] Ignoring out-of-phase message from %s phase: %d block: %d\n", _payload.From, _payload.Phase, _payload.Block)
				continue
			} else if _payload.Phase == 1 {
				var payload Phase1Payload
				err := json.Unmarshal([]byte(data), &payload)
				if err != nil {
					log.Println("Error unmarshalling JSON:", err)
					continue
				}

				// keep it outside goroutine for thread-safety of map
				channels := channelsFactory(payload.Block, payload.Phase, channelsMap, func(block int) {
					triggerPhase2(block, node, topic)
				})

				conds := condsFactory(payload.Block, payload.Phase, condsMap)
				go func() {
					consumePhase1(node, payload, from, datastore, channels, conds)
				}()
			} else if _payload.Phase == 2 {
				payload := _payload

				// keep it outside goroutine for thread-safety of map
				channels := channelsFactory(payload.Block, payload.Phase, channelsMap, func(block int) {
					triggerPhase3(block, node, topic, datastore)
				})

				conds := condsFactory(payload.Block, payload.Phase, condsMap)
				condsPrev := condsFactory(payload.Block, payload.Phase-1, condsMap)
				go func() {
					consumePhase2(node, payload, from, datastore, channels, conds, condsPrev)
				}()
			} else if _payload.Phase == 3 {
				var payload Phase3Payload
				err := json.Unmarshal([]byte(data), &payload)
				if err != nil {
					log.Println("Error unmarshalling JSON:", err)
					continue
				}

				// keep it outside goroutine for thread-safety of map
				channels := channelsFactory(payload.Block, payload.Phase, channelsMap, func(block int) {
					triggerPhase4(block, node, topic, datastore)
				})

				conds := condsFactory(payload.Block, payload.Phase, condsMap)
				condsPrev := condsFactory(payload.Block, payload.Phase-1, condsMap)
				go func() {
					consumePhase3(node, payload, from, datastore, channels, conds, condsPrev)
				}()
			} else if _payload.Phase == 4 {
				var payload Phase4Payload
				err := json.Unmarshal([]byte(data), &payload)
				if err != nil {
					log.Println("Error unmarshalling JSON:", err)
					continue
				}

				// keep it outside goroutine for thread-safety of map
				channels := channelsFactory(payload.Block, payload.Phase, channelsMap, func(block int) {
					triggerPhase5(block, node, topic, datastore)
				})

				conds := condsFactory(payload.Block, payload.Phase, condsMap)
				condsPrev := condsFactory(payload.Block, payload.Phase-1, condsMap)
				go func() {
					consumePhase4(node, payload, from, datastore, channels, conds, condsPrev)
				}()
			} else if _payload.Phase == 5 {
				var payload Phase5Payload
				err := json.Unmarshal([]byte(data), &payload)
				if err != nil {
					log.Println("Error unmarshalling JSON:", err)
					continue
				}

				// keep it outside goroutine for thread-safety of map
				channels := channelsFactory(payload.Block, payload.Phase, channelsMap, func(block int) {
					triggerPhase6(block, node, topic, datastore)
				})

				conds := condsFactory(payload.Block, payload.Phase, condsMap)
				condsPrev := condsFactory(payload.Block, payload.Phase-1, condsMap)
				go func() {
					consumePhase5(node, payload, from, datastore, channels, conds, condsPrev)
				}()
			} else if _payload.Phase == 6 {
				var payload Phase6Payload
				err := json.Unmarshal([]byte(data), &payload)
				if err != nil {
					log.Println("Error unmarshalling JSON:", err)
					continue
				}

				// keep it outside goroutine for thread-safety of map
				channels := channelsFactory(payload.Block, payload.Phase, channelsMap, func(block int) {
					randaoConsensus(block, datastore)
				})

				conds := condsFactory(payload.Block, payload.Phase, condsMap)
				condsPrev := condsFactory(payload.Block, payload.Phase-1, condsMap)
				go func() {
					consumePhase6(node, payload, from, datastore, channels, conds, condsPrev)
				}()
			} else {
				log.Fatalf("[ERROR] Unhandled phase value: %d (%d)\n", _payload.Phase, _payload.Block)
			}
		}
	}(datastore)
}

func InitConsensusLoop(node host.Host, ps *pubsub.PubSub, datastore ds.Datastore) {
	validators := GetValidatorsList()
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
					triggerPhase1(entropy, nextBlockNumber, data, topic, datastore)
				}()
			} else {
				time.Sleep(time.Duration(waitTimeMsec) * time.Millisecond)
			}
		}
	}()
}
