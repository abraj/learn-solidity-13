package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"libp2pdemo/shared"
	"libp2pdemo/utils"
	"log"
	"math"
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

type RandaoPhasePayload struct {
	From  string `json:"from"`
	Block uint64 `json:"block"`
	Phase uint64 `json:"phase"`
}

type RandaoPhase1Payload struct {
	From  string `json:"from"`
	Block uint64 `json:"block"`
	Phase uint64 `json:"phase"`
	Hash  string `json:"hash"`
}

type RandaoPhase3Payload struct {
	From    string `json:"from"`
	Block   uint64 `json:"block"`
	Phase   uint64 `json:"phase"`
	Entropy string `json:"entropy"`
	Salt    string `json:"salt"`
}

type RandaoPhase4Payload struct {
	From  string `json:"from"`
	Block uint64 `json:"block"`
	Phase uint64 `json:"phase"`
	View  string `json:"view"`
}

type RandaoPhase5Payload struct {
	From                 string `json:"from"`
	Block                uint64 `json:"block"`
	Phase                uint64 `json:"phase"`
	ValidatorsMerkleHash string `json:"validatorsMerkleHash"`
	Elapsed              uint64 `json:"elapsed"`
	View                 string `json:"view"`
}

type RandaoPhase6Payload struct {
	From   string `json:"from"`
	Block  uint64 `json:"block"`
	Phase  uint64 `json:"phase"`
	Seed   string `json:"seed"`   // accumulated entropy from contributions by validators
	Output string `json:"output"` // output from VDF delay function
	Delay  uint64 `json:"delay"`  // delay in ms
	Steps  uint64 `json:"steps"`  // number of steps for VDF delay function
}

type RandaoPhaseData struct {
	From string `json:"from"`
	// Delayed bool   `json:"delayed"`
}

type RandaoPhase1Data struct {
	From    string `json:"from"`
	Hash    string `json:"hash"`
	Delayed bool   `json:"delayed"`
}

type RandaoPhase3Data struct {
	From    string `json:"from"`
	Entropy string `json:"entropy"`
	Salt    string `json:"salt"`
	Delayed bool   `json:"delayed"`
}

type RandaoPhase4Data struct {
	From string `json:"from"`
	View string `json:"view"`
	// Delayed bool   `json:"delayed"`
}

type RandaoPhase5Data struct {
	From                 string `json:"from"`
	ValidatorsMerkleHash string `json:"validatorsMerkleHash"`
	Elapsed              uint64 `json:"elapsed"`
	View                 string `json:"view"`
}

type RandaoPhase6Data struct {
	From       string `json:"from"`
	Seed       string `json:"seed"`
	Output     string `json:"output"`
	Steps      uint64 `json:"steps"`
	IsVerified bool   `json:"isVerified"`
}

type RandaoPhaseStatus struct {
	Entropy        string `json:"entropy"`
	Salt           string `json:"salt"`
	PhaseCompleted uint64 `json:"phase"`
	Timeout        bool   `json:"timeout"`
}

type RandaoVotes1 struct {
	Votes      uint64 `json:"votes"`
	VoteMap    string `json:"voteMap"`
	ElapsedMap string `json:"elapsedMap"`
}

type RandaoVotes2 struct {
	Votes        uint64            `json:"votes"`
	EntropyVotes map[string]uint64 `json:"entropyVotes"`
	EntropyMap   map[string]string `json:"entropyMap"`
}

type RandaoConsensusObj struct {
	InFavor           []string `json:"inFavor"`
	Against           []string `json:"against"`
	Elapsed           uint64   `json:"elapsed"`
	InvalidValidators []string `json:"invalidValidators"`
	RandValue         string   `json:"randValue"`
}

type RandaoHashEntropy struct {
	hash    string
	entropy string
	salt    string
}

type Channels struct {
	mutex  *sync.Mutex
	quorum chan uint64
}

type Conds struct {
	cond *sync.Cond
}

const randaoTopic = "baadal-randao"
const randaoPrefix = "/randao/block"

const RANDAO_START_BOUNDARY = 1000 // 1s mark
const RANDAO_DEBUG_LOGS = false

func condsFactory(block uint64, phase uint64, condsMap map[string]Conds) Conds {
	key := fmt.Sprintf("/%d/phase%d", block, phase)
	if condsMap[key].cond == nil {
		var mu sync.Mutex
		cond := sync.NewCond(&mu)
		condsMap[key] = Conds{cond: cond}
	}
	return condsMap[key]
}

func releaseAllConds(condsMap map[string]Conds) {
	for _, conds := range condsMap {
		conds.cond.Broadcast()
	}
}

func channelsFactory(block uint64, phase uint64, datastore ds.Datastore, channelsMap map[string]Channels, condsMap map[string]Conds, onQuorum func(block uint64)) Channels {
	key := fmt.Sprintf("/%d/phase%d", block, phase)
	if channelsMap[key].quorum == nil {
		timer := time.NewTimer(time.Duration(shared.PHASE_DURATION) * time.Millisecond)

		// var mutex sync.Mutex
		mutex := sync.Mutex{}
		quorum := make(chan uint64)

		// mutex not needed (since no parallel goroutines) for `channelsFactory` execution
		channelsMap[key] = Channels{mutex: &mutex, quorum: quorum}

		go func() {
			defer timer.Stop()
			defer close(quorum)
			select {
			case block := <-quorum:
				conds := condsFactory(block, phase, condsMap)
				conds.cond.Broadcast()

				if block == 0 {
					// impasse (no consensus)
					fmt.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>> no consensus")
				} else {
					// quorum achieved
					go func() {
						onQuorum(block)
					}()
				}
			case <-timer.C:
				statusObj, _ := getRandaoStatusObj(block, datastore)
				statusObj.Timeout = true
				setRandaoStatusObj(statusObj, block, datastore)

				// release waiting conds (for all phases)
				releaseAllConds(condsMap)

				// timeout
				fmt.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>> timeout")
			}
		}()
	}
	return channelsMap[key]
}

// NOTE: The validator function is called while receiving (non-local) messages
// as well as _sending_ messages.
func randaoMsgValidator(msg *pubsub.Message) bool {
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

	var payload RandaoPhasePayload
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
		log.Printf("Invalid block number: %d (%d, %d) [%v %v]\n", payload.Block, slotNumber, timeLeftMsec, cond1, cond2)
		return false
	}

	if !utils.Contains([]uint64{1, 2, 3, 4, 5, 6}, payload.Phase) {
		log.Printf("Invalid phase value: %d (%d)\n", payload.Phase, payload.Block)
		return false
	}

	return true
}

func randaoPhase1Payload(node host.Host, blockNumber uint64, entropy string, salt string) string {
	hash := utils.CreateSHA3Hash(entropy, salt)

	from := node.ID().String()
	payload := RandaoPhase1Payload{From: from, Block: blockNumber, Phase: 1, Hash: hash}
	payloadData, _ := json.Marshal(payload)

	payloadStr := string(payloadData)
	return payloadStr
}

func randaoPhase2Payload(node host.Host, blockNumber uint64) string {
	from := node.ID().String()
	payload := RandaoPhasePayload{From: from, Block: blockNumber, Phase: 2}
	payloadData, _ := json.Marshal(payload)

	payloadStr := string(payloadData)
	return payloadStr
}

func randaoPhase3Payload(node host.Host, blockNumber uint64, datastore ds.Datastore) string {
	statusObj, err := getRandaoStatusObj(blockNumber, datastore)
	if err != nil {
		return ""
	}

	from := node.ID().String()
	payload := RandaoPhase3Payload{From: from, Block: blockNumber, Phase: 3, Entropy: statusObj.Entropy, Salt: statusObj.Salt}
	payloadData, _ := json.Marshal(payload)

	payloadStr := string(payloadData)
	return payloadStr
}

func randaoPhase4Payload(node host.Host, blockNumber uint64, datastore ds.Datastore) string {
	prefix := randaoPrefix + fmt.Sprintf("/%d/phase3", blockNumber)
	_, values, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return ""
	}

	viewItems := []string{}
	for _, value := range values {
		var p3Obj RandaoPhase3Data
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
	payload := RandaoPhase4Payload{From: from, Block: blockNumber, Phase: 4, View: string(view)}
	payloadData, _ := json.Marshal(payload)

	payloadStr := string(payloadData)
	return payloadStr
}

func randaoPhase5Payload(node host.Host, blockNumber uint64, datastore ds.Datastore) string {
	validators, err := getValidatorsFromStore(blockNumber, datastore)
	if err != nil {
		return ""
	}

	voteMap := make(map[string]uint64, len(validators))
	partisanMap := make(map[string]bool, len(validators))
	for _, validator := range validators {
		voteMap[validator.String()] = 0
		partisanMap[validator.String()] = false
	}

	partisanMap[node.ID().String()] = true

	// handle tally for current node
	{
		prefix := randaoPrefix + fmt.Sprintf("/%d/phase3", blockNumber)
		_, values, err := QueryWithPrefix(datastore, prefix)
		if err != nil {
			log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
			return ""
		}

		viewItems := []string{}
		for _, value := range values {
			var p3Obj RandaoPhase3Data
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
		prefix := randaoPrefix + fmt.Sprintf("/%d/phase4", blockNumber)
		_, values, err := QueryWithPrefix(datastore, prefix)
		if err != nil {
			log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
			return ""
		}

		for _, value := range values {
			var p4Obj RandaoPhase4Data
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
		if ok && value*2 >= uint64(len(validators)) {
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
	timeElapsedMsec := shared.SLOT_DURATION - RANDAO_START_BOUNDARY - timeLeftMsec

	validatorsList := utils.Map(validators, func(v peer.ID) string {
		return v.String()
	})
	validatorsMerkleHash := utils.MerkleHash(validatorsList)

	from := node.ID().String()
	payload := RandaoPhase5Payload{From: from, Block: blockNumber, Phase: 5, ValidatorsMerkleHash: validatorsMerkleHash, Elapsed: timeElapsedMsec, View: string(view)}
	payloadData, _ := json.Marshal(payload)

	payloadStr := string(payloadData)
	return payloadStr
}

func randaoPhase6Payload(node host.Host, blockNumber uint64, datastore ds.Datastore) string {
	validators, err := getValidatorsFromStore(blockNumber, datastore)
	if err != nil {
		return ""
	}

	hashMap := make(map[string]RandaoHashEntropy)
	for _, validator := range validators {
		item := validator.String()
		hashMap[item] = RandaoHashEntropy{}
	}

	{
		prefix := randaoPrefix + fmt.Sprintf("/%d/phase1", blockNumber)
		_, values, err := QueryWithPrefix(datastore, prefix)
		if err != nil {
			log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
			return ""
		}
		for _, value := range values {
			var p1Obj RandaoPhase1Data
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
			hashMap[item] = tuple
		}
	}

	{
		prefix := randaoPrefix + fmt.Sprintf("/%d/phase3", blockNumber)
		_, values, err := QueryWithPrefix(datastore, prefix)
		if err != nil {
			log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
			return ""
		}
		for _, value := range values {
			var p3Obj RandaoPhase3Data
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
			tuple.salt = p3Obj.Salt
			hashMap[item] = tuple
		}
	}

	consensusKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/consensus", blockNumber))
	consensusData, err := datastore.Get(context.Background(), consensusKey)
	if err != nil {
		log.Println(fmt.Sprintf("Error getting data for key: %s", consensusKey), err)
		return ""
	}

	var consensusObj RandaoConsensusObj
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
			log.Printf("entropy/hash does not exist in hashMap for key: %s (%s, %s, %s)\n", item, value.hash, value.entropy, value.salt)
			return ""
		}

		// verify commit reveal (again)
		hash := utils.CreateSHA3Hash(value.entropy, value.salt)
		if hash == "" || hash != value.hash {
			log.Printf("Unable to verify commitment for key: %s (%s, %s, %s)\n", item, value.hash, value.entropy, value.salt)
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
	y := v.Delay(int64(t), x) // VDF delay
	t2 := time.Now()

	if y == nil {
		log.Printf("[VDF] Unable to create VDF output! Possibly unsupported prime (p ≢ 3 mod 4) used. x: %v y: %v\n", x, y)
		log.Printf("[VDF] Prime used: %v\n", v.GetModulus())
		return ""
	} else {
		if !v.Verify(int64(t), x, y) {
			log.Printf("[VDF] Unable to verify VDF output! Input (seed) to VDF possibly too large. x: %v y: %v\n", x, y)
			log.Printf("[VDF] Prime used: %v\n", v.GetModulus())
			return ""
		}
	}

	output := y.Text(16)
	delay := uint64(t2.Sub(t1).Milliseconds())

	from := node.ID().String()
	payload := RandaoPhase6Payload{From: from, Block: blockNumber, Phase: 6, Seed: composedHash, Output: output, Delay: delay, Steps: t}
	payloadData, _ := json.Marshal(payload)

	payloadStr := string(payloadData)
	return payloadStr
}

func initBlockData(blockNumber uint64, datastore ds.Datastore) []peer.ID {
	validators := GetValidatorsList()
	validatorsList, err := json.Marshal(validators)
	if err != nil {
		log.Println("Error marshalling validators list:", err)
		return nil
	}
	validatorsListStr := string(validatorsList)

	validatorsKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/validators", blockNumber))
	if err := datastore.Put(context.Background(), validatorsKey, []byte(validatorsListStr)); err != nil {
		log.Println(fmt.Sprintf("Error setting data for key: %s", validatorsKey), err)
		return nil
	}

	votesObjData, _ := json.Marshal(RandaoVotes1{Votes: 0, VoteMap: "{}", ElapsedMap: "{}"})
	votesKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/votes", blockNumber))
	if err := datastore.Put(context.Background(), votesKey, []byte(votesObjData)); err != nil {
		log.Println(fmt.Sprintf("Error setting data for key: %s", votesKey), err)
		return nil
	}

	rvotesObjData, _ := json.Marshal(RandaoVotes2{Votes: 0, EntropyVotes: map[string]uint64{}, EntropyMap: map[string]string{}})
	rvotesKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/randao-votes", blockNumber))
	if err := datastore.Put(context.Background(), rvotesKey, []byte(rvotesObjData)); err != nil {
		log.Println(fmt.Sprintf("Error setting data for key: %s", rvotesKey), err)
		return nil
	}

	return validators
}

// phase1: commit phase
func triggerRandaoPhase1(entropy string, salt string, blockNumber uint64, data string, topic *pubsub.Topic, datastore ds.Datastore) {
	// if RANDAO_DEBUG_LOGS {
	fmt.Println("triggerRandaoPhase1..", blockNumber)
	// }

	latestBlockNumber := shared.GetLatestBlockNumber()
	if blockNumber != latestBlockNumber {
		log.Fatalf("[PANIC] Randao phase called for invalid block number: %d %d\n", blockNumber, latestBlockNumber)
	}

	// var payload RandaoPhase1Payload
	// err := json.Unmarshal([]byte(data), &payload)
	// if err != nil {
	// 	log.Println("Error unmarshalling JSON:", err)
	// 	return
	// }

	statusObj, err := getRandaoStatusObj(blockNumber, datastore)
	if err != nil {
		return
	}

	statusObj.Entropy = entropy
	statusObj.Salt = salt
	err = setRandaoStatusObj(statusObj, blockNumber, datastore)
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
func triggerRandaoPhase2(block uint64, node host.Host, topic *pubsub.Topic) {
	if RANDAO_DEBUG_LOGS {
		fmt.Println("triggerRandaoPhase2..", block)
	}

	data := randaoPhase2Payload(node, block)
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
func triggerRandaoPhase3(block uint64, node host.Host, topic *pubsub.Topic, datastore ds.Datastore) {
	if RANDAO_DEBUG_LOGS {
		fmt.Println("triggerRandaoPhase3..", block)
	}

	data := randaoPhase3Payload(node, block, datastore)
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
func triggerRandaoPhase4(block uint64, node host.Host, topic *pubsub.Topic, datastore ds.Datastore) {
	if RANDAO_DEBUG_LOGS {
		fmt.Println("triggerRandaoPhase4..", block)
	}

	data := randaoPhase4Payload(node, block, datastore)
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
func triggerRandaoPhase5(block uint64, node host.Host, topic *pubsub.Topic, datastore ds.Datastore) {
	if RANDAO_DEBUG_LOGS {
		fmt.Println("triggerRandaoPhase5..", block)
	}

	data := randaoPhase5Payload(node, block, datastore)
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
func triggerRandaoPhase6(block uint64, node host.Host, topic *pubsub.Topic, datastore ds.Datastore) {
	if RANDAO_DEBUG_LOGS {
		fmt.Println("triggerRandaoPhase6..", block)
	}

	data := randaoPhase6Payload(node, block, datastore)
	if data == "" {
		return
	}

	err := topic.Publish(context.Background(), []byte(data))
	if err != nil {
		log.Printf("Could not publish invalid message: %v\n", err)
		// panic(err)
	}
}

func consumeRandaoMix(block uint64, datastore ds.Datastore) {
	if RANDAO_DEBUG_LOGS {
		fmt.Println("consumeRandaoMix..", block)
	}

	consensusKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/consensus", block))
	consensusData, _ := datastore.Get(context.Background(), consensusKey)

	var consensusObj RandaoConsensusObj
	json.Unmarshal(consensusData, &consensusObj)

	randaoMix := consensusObj.RandValue
	// fmt.Println("InFavor:", consensusObj.InFavor)

	// if RANDAO_DEBUG_LOGS {
	fmt.Println("randaoMix:", randaoMix)
	// }
}

func getRandaoStatusObj(block uint64, datastore ds.Datastore) (RandaoPhaseStatus, error) {
	statusKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/status", block))
	statusData, err := datastore.Get(context.Background(), statusKey)
	if err != nil {
		statusObj := RandaoPhaseStatus{PhaseCompleted: 0}
		if err := setRandaoStatusObj(statusObj, block, datastore); err == nil {
			return statusObj, nil
		}
		log.Println(fmt.Sprintf("Error getting data for key: %s", statusKey), err)
		return RandaoPhaseStatus{}, err
	}

	var statusObj RandaoPhaseStatus
	err = json.Unmarshal(statusData, &statusObj)
	if err != nil {
		log.Println("Error unmarshalling status data:", err)
		return RandaoPhaseStatus{}, err
	}

	return statusObj, nil
}

func setRandaoStatusObj(statusObj RandaoPhaseStatus, block uint64, datastore ds.Datastore) error {
	statusData, err := json.Marshal(statusObj)
	if err != nil {
		log.Println("Error marshalling status data:", err)
		return err
	}

	statusKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/status", block))
	if err := datastore.Put(context.Background(), statusKey, statusData); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", statusKey), err)
		return err
	}

	return nil
}

func getValidatorsFromStore(block uint64, datastore ds.Datastore) ([]peer.ID, error) {
	validatorsKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/validators", block))
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

func consumeRandaoPhase1(node host.Host, payload RandaoPhase1Payload, from peer.ID, datastore ds.Datastore, channels Channels) {
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

	channels.mutex.Lock()
	defer channels.mutex.Unlock()

	statusObj, err := getRandaoStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}
	if statusObj.Timeout {
		return
	}

	phase1Completed := statusObj.PhaseCompleted >= 1

	p1Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	_, err = datastore.Get(context.Background(), p1Key)
	if err == nil {
		log.Printf("Value already exists in datastore for key: %s", p1Key)
		return
	}

	data := RandaoPhase1Data{From: payload.From, Hash: payload.Hash, Delayed: phase1Completed}
	p1Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// move below to avoid capturing laggard/delayed messages
	p1DataStr := string(p1Data)
	// p1Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
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
	prefix := randaoPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, _, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	allKeys := utils.Map(keys, func(key ds.Key) string {
		return key.String()
	})
	hostKey := randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, node.ID().String())
	selfAccountedFor := utils.Contains(allKeys, hostKey)

	if RANDAO_DEBUG_LOGS {
		fmt.Println("<---- prefix:", len(validators), len(keys), prefix, from)
		// fmt.Println("keys:", keys)
		// fmt.Println("values:", values)
	}

	// wait for 90th percentile response
	if selfAccountedFor && len(keys)*10 >= len(validators)*9 {
		statusObj.PhaseCompleted = 1
		err := setRandaoStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}

		// close phase1
		channels.quorum <- payload.Block
	}

	// TODO: cleanup memory store, datastore, etc. for each /block-number
}

func consumeRandaoPhase2(node host.Host, payload RandaoPhasePayload, from peer.ID, datastore ds.Datastore, channels Channels, condsPrev Conds) {
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

	statusObj, err := getRandaoStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}
	if statusObj.Timeout {
		return
	}

	if statusObj.PhaseCompleted < 1 {
		condsPrev.cond.L.Lock()
		condsPrev.cond.Wait()
		condsPrev.cond.L.Unlock()
	}

	statusObj, _ = getRandaoStatusObj(payload.Block, datastore)
	if statusObj.Timeout {
		return
	}

	phase2Completed := statusObj.PhaseCompleted >= 2

	if phase2Completed {
		// log.Printf("Phase 2 already completed. Status: %d\n", statusObj.PhaseCompleted)
		return
	}

	p2Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	_, err = datastore.Get(context.Background(), p2Key)
	if err == nil {
		log.Printf("Value already exists in datastore for key: %s", p2Key)
		return
	}

	// `from` must be in committed set from phase1
	p1Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase1/%s", payload.Block, from))
	p1MsgData, err := datastore.Get(context.Background(), p1Key)
	if err != nil {
		// log.Println(fmt.Sprintf("Error getting value from datastore for key: %s", p1Key), err)
		return
	}

	var p1Msg RandaoPhase1Data
	err = json.Unmarshal(p1MsgData, &p1Msg)
	if err != nil {
		log.Printf("Error unmarshalling phase1 message: %v", err)
		return
	}

	// only consider if part of phase1 cut-off
	if p1Msg.Delayed {
		return
	}

	data := RandaoPhaseData{From: payload.From /*, Delayed: phase2Completed*/}
	p2Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// NOTE: currently, only valid (part of phase1 cut-off and already committed) phase2 messages are being stored
	// Also, stop storing messages as soon as (phase2) cut-off is reached
	// p2Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	if err := datastore.Put(context.Background(), p2Key, p2Data); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", p2Key), err)
		return
	}

	// query data needed for deciding progress
	prefix := randaoPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, _, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	allKeys := utils.Map(keys, func(key ds.Key) string {
		return key.String()
	})
	hostKey := randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, node.ID().String())
	selfAccountedFor := utils.Contains(allKeys, hostKey)

	if RANDAO_DEBUG_LOGS {
		fmt.Println("<---- prefix:", len(validators), len(keys), prefix, from)
	}

	// wait for 90th percentile response (among committed set from phase1)
	if selfAccountedFor && len(keys)*100 >= len(validators)*81 {
		statusObj.PhaseCompleted = 2
		err := setRandaoStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}

		// close phase2
		channels.quorum <- payload.Block
	}
}

func consumeRandaoPhase3(node host.Host, payload RandaoPhase3Payload, from peer.ID, datastore ds.Datastore, channels Channels, condsPrev Conds) {
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

	// Salt: [32 len hex] 16 bytes
	if len(payload.Salt) != 32 {
		log.Printf("Invalid salt: %s (%s)\n", payload.Salt, from)
		return
	}

	channels.mutex.Lock()
	defer channels.mutex.Unlock()

	statusObj, err := getRandaoStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}
	if statusObj.Timeout {
		return
	}

	if statusObj.PhaseCompleted < 2 {
		condsPrev.cond.L.Lock()
		condsPrev.cond.Wait()
		condsPrev.cond.L.Unlock()
	}

	statusObj, _ = getRandaoStatusObj(payload.Block, datastore)
	if statusObj.Timeout {
		return
	}

	phase3Completed := statusObj.PhaseCompleted >= 3

	p3Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	_, err = datastore.Get(context.Background(), p3Key)
	if err == nil {
		log.Printf("Value already exists in datastore for key: %s", p3Key)
		return
	}

	p1Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase1/%s", payload.Block, from))
	p1MsgData, err := datastore.Get(context.Background(), p1Key)
	if err != nil {
		// log.Println(fmt.Sprintf("Error getting value from datastore for key: %s", p1Key), err)
		return
	}

	var p1Msg RandaoPhase1Data
	err = json.Unmarshal(p1MsgData, &p1Msg)
	if err != nil {
		log.Printf("Error unmarshalling phase1 message: %v", err)
		return
	}

	// verify commit reveal
	hash := utils.CreateSHA3Hash(payload.Entropy, payload.Salt)
	if hash == "" || hash != p1Msg.Hash {
		log.Printf("Unable to verify commitment: %s (%s %s)\n", p1Msg.Hash, payload.Entropy, payload.Salt)
		return
	}

	data := RandaoPhase3Data{From: payload.From, Entropy: payload.Entropy, Salt: payload.Salt, Delayed: p1Msg.Delayed}
	p3Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// NOTE: keep storing phase3 messages even after 3/4 majority
	// Assumption: phase3 message for a node (for a block number) arrives after phase1 message
	// NOTE: phase3 message for a node is considered delayed if the corresponding phase1 message is delayed
	p3DataStr := string(p3Data)
	// p3Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	if err := datastore.Put(context.Background(), p3Key, []byte(p3DataStr)); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", p3Key), err)
		return
	}

	if phase3Completed {
		// log.Printf("Phase 3 already completed. Status: %d\n", statusObj.PhaseCompleted)
		return
	}

	// query data needed for deciding progress
	prefix := randaoPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, _, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	allKeys := utils.Map(keys, func(key ds.Key) string {
		return key.String()
	})
	hostKey := randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, node.ID().String())
	selfAccountedFor := utils.Contains(allKeys, hostKey)

	if RANDAO_DEBUG_LOGS {
		fmt.Println("<---- prefix:", len(validators), len(keys), prefix, from)
	}

	// wait for 3/4 responses
	if selfAccountedFor && len(keys)*4 >= len(validators)*3 {
		statusObj.PhaseCompleted = 3
		err := setRandaoStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}

		// close phase3
		channels.quorum <- payload.Block
	}
}

func consumeRandaoPhase4(node host.Host, payload RandaoPhase4Payload, from peer.ID, datastore ds.Datastore, channels Channels, condsPrev Conds) {
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

	statusObj, err := getRandaoStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}
	if statusObj.Timeout {
		return
	}

	if statusObj.PhaseCompleted < 3 {
		condsPrev.cond.L.Lock()
		condsPrev.cond.Wait()
		condsPrev.cond.L.Unlock()
	}

	statusObj, _ = getRandaoStatusObj(payload.Block, datastore)
	if statusObj.Timeout {
		return
	}

	phase4Completed := statusObj.PhaseCompleted >= 4

	p4Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	_, err = datastore.Get(context.Background(), p4Key)
	if err == nil {
		log.Printf("Value already exists in datastore for key: %s", p4Key)
		return
	}

	data := RandaoPhase4Data{From: payload.From, View: payload.View /*, Delayed: phase4Completed*/}
	p4Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// NOTE: keep storing phase4 messages
	p4DataStr := string(p4Data)
	// p4Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	if err := datastore.Put(context.Background(), p4Key, []byte(p4DataStr)); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", p4Key), err)
		return
	}

	if phase4Completed {
		// log.Printf("Phase 4 already completed. Status: %d\n", statusObj.PhaseCompleted)
		return
	}

	// query data needed for deciding progress
	prefix := randaoPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, _, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	allKeys := utils.Map(keys, func(key ds.Key) string {
		return key.String()
	})
	hostKey := randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, node.ID().String())
	selfAccountedFor := utils.Contains(allKeys, hostKey)

	if RANDAO_DEBUG_LOGS {
		fmt.Println("<---- prefix:", len(validators), len(keys), prefix, from)
	}

	// wait for 90th percentile response
	if selfAccountedFor && len(keys)*10 >= len(validators)*9 {
		statusObj.PhaseCompleted = 4
		err := setRandaoStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}

		// close phase4
		channels.quorum <- payload.Block
	}
}

func consumeRandaoPhase5(node host.Host, payload RandaoPhase5Payload, from peer.ID, datastore ds.Datastore, channels Channels, condsPrev Conds, isValidator bool) {
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

	statusObj, err := getRandaoStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}
	if statusObj.Timeout {
		return
	}

	if isValidator && statusObj.PhaseCompleted < 4 {
		condsPrev.cond.L.Lock()
		condsPrev.cond.Wait()
		condsPrev.cond.L.Unlock()
	}

	statusObj, _ = getRandaoStatusObj(payload.Block, datastore)
	if statusObj.Timeout {
		return
	}

	phase5Completed := statusObj.PhaseCompleted >= 5

	if phase5Completed {
		// log.Printf("Phase 5 already completed. Status: %d\n", statusObj.PhaseCompleted)
		return
	}

	p5Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	_, err = datastore.Get(context.Background(), p5Key)
	if err == nil {
		log.Printf("Value already exists in datastore for key: %s", p5Key)
		return
	}

	data := RandaoPhase5Data{From: payload.From, ValidatorsMerkleHash: payload.ValidatorsMerkleHash, Elapsed: payload.Elapsed, View: payload.View}
	p5Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// store phase5 response messages
	p5DataStr := string(p5Data)
	// p5Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
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
	prefix := randaoPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, values, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	invalidValidators := []string{}
	for _, value := range values {
		var p5Data RandaoPhase5Data
		json.Unmarshal([]byte(value), &p5Data)
		isValidatorsMatch := p5Data.ValidatorsMerkleHash == validatorsMerkleHash
		if !isValidatorsMatch {
			invalidValidators = append(invalidValidators, p5Data.From)
		}
	}

	if len(invalidValidators)*3 > len(validators) {
		log.Printf("Possibly current node's validator list not up-to-date: %v\n", validators)
		log.Printf("Too many invalid validators: %d (%d)\n", len(invalidValidators), len(validators))
		return
	}
	validValidatorsLen := uint64(len(validators) - len(invalidValidators))

	votesKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/votes", payload.Block))
	votesData, err := datastore.Get(context.Background(), votesKey)
	if err != nil {
		log.Println(fmt.Sprintf("Error getting data for key: %s", votesKey), err)
		return
	}

	var votesObj RandaoVotes1
	err = json.Unmarshal(votesData, &votesObj)
	if err != nil {
		log.Printf("[ERROR] Unable to unmarshal votes data: %s\n", string(votesData))
		return
	}

	voteMap := make(map[string]uint64)
	err = json.Unmarshal([]byte(votesObj.VoteMap), &voteMap)
	if err != nil {
		log.Printf("[ERROR] Unable to unmarshal voteMap data: %s\n", votesObj.VoteMap)
		return
	}

	elapsedMap := make(map[string]uint64)
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

	if RANDAO_DEBUG_LOGS {
		fmt.Println("<---- data:", len(validators), votesObj.Votes, "phase5", data.From)
	}

	voteMapData, _ := json.Marshal(voteMap)
	elapsedMapData, _ := json.Marshal(elapsedMap)
	votesObj.VoteMap = string(voteMapData)
	votesObj.ElapsedMap = string(elapsedMapData)

	votesObjData, err := json.Marshal(votesObj)
	if err != nil {
		fmt.Printf("[WARN] Unable to marshal votesObj: %v\n", votesObj)
		return
	}

	// votesKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/votes", payload.Block))
	if err := datastore.Put(context.Background(), votesKey, []byte(votesObjData)); err != nil {
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
	elapsedList := []uint64{}
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

	var retVal int64 = -1
	if consensus {
		// consensus
		retVal = int64(payload.Block)

		// save consensus data
		elapsed := utils.Median(elapsedList)
		consensusObj := RandaoConsensusObj{InFavor: inFavorList, Against: againstList, Elapsed: elapsed, InvalidValidators: invalidValidators}
		consensusObjData, _ := json.Marshal(consensusObj)
		consensusKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/consensus", payload.Block))
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
	hostKey := randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, node.ID().String())
	selfAccountedFor := utils.Contains(allKeys, hostKey)

	if (!isValidator || selfAccountedFor) && retVal >= 0 {
		statusObj.PhaseCompleted = 5
		err := setRandaoStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}

		// close phase5
		channels.quorum <- uint64(retVal)
	}
}

func consumeRandaoPhase6(node host.Host, payload RandaoPhase6Payload, from peer.ID, datastore ds.Datastore, channels Channels, condsPrev Conds, isValidator bool) {
	validators, err := getValidatorsFromStore(payload.Block, datastore)
	if err != nil {
		return
	}

	if includes := utils.IsValidator(from, validators); !includes {
		log.Printf("[WARN] Invalid validator: %s\n", from)
		return
	}

	consensusKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/consensus", payload.Block))
	consensusData, err := datastore.Get(context.Background(), consensusKey)
	if err != nil {
		log.Println(fmt.Sprintf("Error getting data for key: %s", consensusKey), err)
		return
	}

	var consensusObj RandaoConsensusObj
	err = json.Unmarshal(consensusData, &consensusObj)
	if err != nil {
		log.Printf("Unable to unmarshal consensus data: %s %v\n", consensusData, err)
		return
	}

	invalidValidators := consensusObj.InvalidValidators
	validValidatorsLen := uint64(len(validators) - len(invalidValidators))

	if utils.Contains(invalidValidators, from.String()) {
		log.Printf("[WARN] Invalid 'validators list' at this validator: %s\n", from)
		return
	}

	channels.mutex.Lock()
	defer channels.mutex.Unlock()

	statusObj, err := getRandaoStatusObj(payload.Block, datastore)
	if err != nil {
		return
	}
	if statusObj.Timeout {
		return
	}

	if statusObj.PhaseCompleted < 5 {
		condsPrev.cond.L.Lock()
		condsPrev.cond.Wait()
		condsPrev.cond.L.Unlock()
	}

	statusObj, _ = getRandaoStatusObj(payload.Block, datastore)
	if statusObj.Timeout {
		return
	}

	phase6Completed := statusObj.PhaseCompleted >= 6

	if phase6Completed {
		// log.Printf("Phase 6 already completed. Status: %d\n", statusObj.PhaseCompleted)
		return
	}

	p6Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	_, err = datastore.Get(context.Background(), p6Key)
	if err == nil {
		log.Printf("Value already exists in datastore for key: %s", p6Key)
		return
	}

	// VDF verify
	t := payload.Steps
	x, _ := new(big.Int).SetString(payload.Seed, 16)
	y, _ := new(big.Int).SetString(payload.Output, 16)
	v := vdf.NewVDF()
	isVerified := v.Verify(int64(t), x, y)

	data := RandaoPhase6Data{From: payload.From, Seed: payload.Seed, Output: payload.Output, Steps: payload.Steps, IsVerified: isVerified}
	p6Data, err := json.Marshal(data)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	// store phase6 response messages
	p6DataStr := string(p6Data)
	// p6Key := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, from))
	if err := datastore.Put(context.Background(), p6Key, []byte(p6DataStr)); err != nil {
		log.Println(fmt.Sprintf("Error putting value in datastore for key: %s", p6Key), err)
		return
	}

	// query saved response messages
	prefix := randaoPrefix + fmt.Sprintf("/%d/phase%d", payload.Block, payload.Phase)
	keys, _, err := QueryWithPrefix(datastore, prefix)
	if err != nil {
		log.Println(fmt.Sprintf("Error querying datastore with prefix: %s", prefix), err)
		return
	}

	rvotesKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/randao-votes", payload.Block))
	rvotesData, err := datastore.Get(context.Background(), rvotesKey)
	if err != nil {
		log.Println(fmt.Sprintf("Error getting data for key: %s", rvotesKey), err)
		return
	}

	var rvotesObj RandaoVotes2
	err = json.Unmarshal(rvotesData, &rvotesObj)
	if err != nil {
		log.Printf("[ERROR] Unable to unmarshal randao-votes data: %s\n", string(rvotesData))
		return
	}

	rvotesObj.Votes += 1

	if RANDAO_DEBUG_LOGS {
		fmt.Println("<---- data:", len(validators), rvotesObj.Votes, "phase6", data.From)
	}

	if !isVerified {
		log.Printf("[ERROR] VDF verification failed! x: %s y: %s t: %d\n", payload.Seed, payload.Output, payload.Steps)
		return
	}

	rKey := utils.CreateSHA3Hash(payload.Seed+payload.Output+strconv.FormatInt(int64(payload.Steps), 10), "")
	eVotes, ok := rvotesObj.EntropyVotes[rKey]
	if ok {
		rvotesObj.EntropyVotes[rKey] = eVotes + 1
	} else {
		rvotesObj.EntropyVotes[rKey] = 1
		rvotesObj.EntropyMap[rKey] = payload.Output
	}

	rvotesObjData, err := json.Marshal(rvotesObj)
	if err != nil {
		fmt.Printf("[WARN] Unable to marshal votesObj: %v\n", rvotesObj)
		return
	}

	// rvotesKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/randao-votes", payload.Block))
	if err := datastore.Put(context.Background(), rvotesKey, []byte(rvotesObjData)); err != nil {
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

	var retVal int64 = -1
	if consensus {
		// consensus
		retVal = int64(payload.Block)

		// save consensus data (randao mix)
		consensusObj.RandValue = consensusRandValue
		consensusObjData, _ := json.Marshal(consensusObj)
		consensusKey := ds.NewKey(randaoPrefix + fmt.Sprintf("/%d/consensus", payload.Block))
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
	hostKey := randaoPrefix + fmt.Sprintf("/%d/phase%d/%s", payload.Block, payload.Phase, node.ID().String())
	selfAccountedFor := utils.Contains(allKeys, hostKey)

	if (!isValidator || selfAccountedFor) && retVal >= 0 {
		statusObj.PhaseCompleted = 6
		err := setRandaoStatusObj(statusObj, payload.Block, datastore)
		if err != nil {
			return
		}

		// close phase6
		channels.quorum <- uint64(retVal)
	}
}

func listenRandaoTopic(node host.Host, topic *pubsub.Topic, datastore ds.Datastore, isValidator bool) {
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

			var _payload RandaoPhasePayload
			err = json.Unmarshal([]byte(data), &_payload)
			if err != nil {
				log.Println("Error unmarshalling JSON:", err)
				continue
			}

			statusObj, err := getRandaoStatusObj(_payload.Block, datastore)
			if err != nil {
				log.Println("Unable to get statusObj:", err)
				continue
			}
			ongoingPhase := statusObj.PhaseCompleted + 1
			maxAllowedPhase := ongoingPhase + 1
			var consensusPhase uint64 = 5 // consensus (5), randao mix (6)

			if !isValidator && _payload.Phase < consensusPhase {
				// only allow last (consensus) phase message for non-validators
				continue
			}

			if isValidator && _payload.Phase > maxAllowedPhase {
				log.Printf("[WARN] Ignoring out-of-phase message from %s phase: %d block: %d\n", _payload.From, _payload.Phase, _payload.Block)
				continue
			} else if _payload.Phase == 1 {
				var payload RandaoPhase1Payload
				err := json.Unmarshal([]byte(data), &payload)
				if err != nil {
					log.Println("Error unmarshalling JSON:", err)
					continue
				}

				// keep it outside goroutine for thread-safety of map
				channels := channelsFactory(payload.Block, payload.Phase, datastore, channelsMap, condsMap, func(block uint64) {
					triggerRandaoPhase2(block, node, topic)
				})

				go func() {
					consumeRandaoPhase1(node, payload, from, datastore, channels)
				}()
			} else if _payload.Phase == 2 {
				payload := _payload

				// keep it outside goroutine for thread-safety of map
				channels := channelsFactory(payload.Block, payload.Phase, datastore, channelsMap, condsMap, func(block uint64) {
					triggerRandaoPhase3(block, node, topic, datastore)
				})

				condsPrev := condsFactory(payload.Block, payload.Phase-1, condsMap)
				go func() {
					consumeRandaoPhase2(node, payload, from, datastore, channels, condsPrev)
				}()
			} else if _payload.Phase == 3 {
				var payload RandaoPhase3Payload
				err := json.Unmarshal([]byte(data), &payload)
				if err != nil {
					log.Println("Error unmarshalling JSON:", err)
					continue
				}

				// keep it outside goroutine for thread-safety of map
				channels := channelsFactory(payload.Block, payload.Phase, datastore, channelsMap, condsMap, func(block uint64) {
					triggerRandaoPhase4(block, node, topic, datastore)
				})

				condsPrev := condsFactory(payload.Block, payload.Phase-1, condsMap)
				go func() {
					consumeRandaoPhase3(node, payload, from, datastore, channels, condsPrev)
				}()
			} else if _payload.Phase == 4 {
				var payload RandaoPhase4Payload
				err := json.Unmarshal([]byte(data), &payload)
				if err != nil {
					log.Println("Error unmarshalling JSON:", err)
					continue
				}

				// keep it outside goroutine for thread-safety of map
				channels := channelsFactory(payload.Block, payload.Phase, datastore, channelsMap, condsMap, func(block uint64) {
					triggerRandaoPhase5(block, node, topic, datastore)
				})

				condsPrev := condsFactory(payload.Block, payload.Phase-1, condsMap)
				go func() {
					consumeRandaoPhase4(node, payload, from, datastore, channels, condsPrev)
				}()
			} else if _payload.Phase == 5 {
				var payload RandaoPhase5Payload
				err := json.Unmarshal([]byte(data), &payload)
				if err != nil {
					log.Println("Error unmarshalling JSON:", err)
					continue
				}

				// keep it outside goroutine for thread-safety of map
				channels := channelsFactory(payload.Block, payload.Phase, datastore, channelsMap, condsMap, func(block uint64) {
					if isValidator {
						triggerRandaoPhase6(block, node, topic, datastore)
					}
				})

				condsPrev := condsFactory(payload.Block, payload.Phase-1, condsMap)
				go func() {
					consumeRandaoPhase5(node, payload, from, datastore, channels, condsPrev, isValidator)
				}()
			} else if _payload.Phase == 6 {
				var payload RandaoPhase6Payload
				err := json.Unmarshal([]byte(data), &payload)
				if err != nil {
					log.Println("Error unmarshalling JSON:", err)
					continue
				}

				// keep it outside goroutine for thread-safety of map
				channels := channelsFactory(payload.Block, payload.Phase, datastore, channelsMap, condsMap, func(block uint64) {
					consumeRandaoMix(block, datastore)
				})

				condsPrev := condsFactory(payload.Block, payload.Phase-1, condsMap)
				go func() {
					consumeRandaoPhase6(node, payload, from, datastore, channels, condsPrev, isValidator)
				}()
			} else {
				log.Fatalf("[ERROR] Unhandled phase value: %d (%d)\n", _payload.Phase, _payload.Block)
			}
		}
	}(datastore)
}

func InitRandaoLoop(node host.Host, ps *pubsub.PubSub, datastore ds.Datastore) {
	validators := GetValidatorsList()
	isValidator := utils.IsValidator(node.ID(), validators)

	payloadValidator := func(ctx context.Context, pid peer.ID, msg *pubsub.Message) bool {
		// NOTE: `pid` could be an intermediary or relaying node, not necessarily the original publisher
		// unlike `msg.GetFrom()` which is the peer ID of the original publisher of the message
		return randaoMsgValidator(msg)
	}
	err := ps.RegisterTopicValidator(randaoTopic, payloadValidator)
	if err != nil {
		panic(err)
	}
	topic, err := ps.Join(randaoTopic)
	if err != nil {
		panic(err)
	}

	// applicable (partially) for non-validators also
	listenRandaoTopic(node, topic, datastore, isValidator)

	if !isValidator {
		// current node is not a validator
		return
	}

	go func() {
		initialCall := true
		_, initialDelayMsec := shared.NextBlockInfo(initialCall)
		additionalDelay := shared.SLOT_DURATION // NOTE: additional time delay to allow network time sync completion
		delay := initialDelayMsec + additionalDelay
		time.Sleep(time.Duration(delay) * time.Millisecond)

		for {
			midpointDelay := shared.SLOT_DURATION / 2    // drift to slot midpoint
			randomDelay := uint64(rand.Float64() * 1000) // 0-1s
			delay := midpointDelay + randomDelay
			time.Sleep(time.Duration(delay) * time.Millisecond)

			nextBlockNumber, waitTimeMsec := shared.NextBlockInfo(initialCall)
			if nextBlockNumber > 0 {
				timer := time.After(time.Duration(waitTimeMsec) * time.Millisecond)
				entropy := utils.RandomHex(32)
				salt := utils.RandomHex(16)
				data := randaoPhase1Payload(node, nextBlockNumber, entropy, salt)
				<-timer // wait for next slot start

				// Move to randao start boundary (1s mark)
				// NOTE: to change or disable start boundary, do not comment the following sleep statement.
				// Rather, update the value of `RANDAO_START_BOUNDARY` constant since it's used at other places also
				time.Sleep(RANDAO_START_BOUNDARY * time.Millisecond)

				timeShiftMsec := shared.NetworkTimeShift()
				if uint64(math.Abs(float64(timeShiftMsec))) > shared.MAX_CLOCK_DRIFT {
					log.Printf("[WARN] Current time drift too large: %d\n", timeShiftMsec)
					// fmt.Printf("Skipping new phase trigger.. [block %d]\n", nextBlockNumber)
					// continue
				}

				if initialCall {
					initialCall = false
				}
				go func() {
					triggerRandaoPhase1(entropy, salt, nextBlockNumber, data, topic, datastore)
				}()
			} else {
				time.Sleep(time.Duration(waitTimeMsec) * time.Millisecond)
			}
		}
	}()
}
