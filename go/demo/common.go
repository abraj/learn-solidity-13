package demo

import (
	"fmt"
	"libp2pdemo/baadal/client"
	"libp2pdemo/baadal/request"
	"libp2pdemo/shared"
	"libp2pdemo/utils"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

func onConnected(node host.Host, peerID peer.ID) {
	protocolID := client.ProtocolID()

	resp := request.RequestService(node, peerID, protocolID, "clientinfo")
	respMap := utils.ResponseMap(resp)
	if utils.ValidateResponse(respMap) {
		// fmt.Println(">>", peerID, "Valid")
	} else {
		EvictPeer(node, peerID)
		fmt.Println(">> Evicted:", peerID)
	}
}

func EvictPeer(node host.Host, peerID peer.ID) {
	// kadDHT.RoutingTable().RemovePeer(peerId)
	// node.ConnManager().TagPeer(peerID, "blocked", -1000)

	node.Peerstore().ClearAddrs(peerID)
	node.Peerstore().RemovePeer(peerID)

	time.Sleep(2 * time.Second) // delay to avoid race condition among nodes
	node.Network().ClosePeer(peerID)
}

func queryTime(node host.Host, peerID peer.ID) (int64, int64) {
	protocolID := client.ProtocolID()

	t0 := shared.NetworkTime() // record the time when the request was sent
	resp := request.RequestService(node, peerID, protocolID, "timestamp")
	t3 := shared.NetworkTime() // record the time when the response was received

	// TODO: send nonce in request
	respMap := utils.ResponseMap(resp)
	// TODO: validate nonce in response
	if utils.ValidateResponse(respMap, "epoch_t1", "epoch_t2") {
		epoch_t1 := respMap["epoch_t1"]
		epoch_t2 := respMap["epoch_t2"]

		t1, err := strconv.ParseInt(epoch_t1, 10, 64)
		if err != nil {
			log.Println("Error converting string to int64:", epoch_t1, err)
			return 0, t0
		}
		t2, err := strconv.ParseInt(epoch_t2, 10, 64)
		if err != nil {
			log.Println("Error converting string to int64:", epoch_t2, err)
			return 0, t0
		}

		roundTripDelay := (t3 - t0) - (t2 - t1)
		delay := roundTripDelay / 2

		return t2 + delay, t0
	}

	return 0, t0
}

func getNetworkTimeShift(node host.Host, validators []peer.ID) int {
	if len(validators) == 0 {
		return 0
	}

	var (
		result = make(map[string]int)
		mu     sync.Mutex
		wg     sync.WaitGroup
	)

	// TODO: randomly select a smaller subset of validators for reduce communication overhead

	for _, validator := range validators {
		wg.Add(1) // Increment the WaitGroup counter

		go func(v peer.ID) {
			defer wg.Done() // Decrement the counter when the goroutine completes

			if node.ID().String() == v.String() {
				// ignore if the current node is in the validator list
				return
			}

			timestamp, t0 := queryTime(node, v)

			if timestamp == 0 {
				// filter invalid or timed out values
				return
			}

			shift := int(timestamp - t0)
			absDiff := shift
			if absDiff < 0 {
				absDiff = -absDiff
			}
			if absDiff > shared.SLOT_DURATION/2 {
				// filter extreme values
				return
			}

			mu.Lock() // maps are not thread-safe in Go
			result[v.String()] = shift
			mu.Unlock()
		}(validator) // Pass `validator` as an argument to avoid closure issues
	}

	wg.Wait() // Wait for all goroutines to finish

	if len(result) == 0 {
		return 0
	}

	timestamps := []int64{}
	for _, value := range result {
		timestamps = append(timestamps, int64(value))
	}

	median := utils.Median(timestamps)
	timeShift := int(median)

	return timeShift
}

func AdjustNetworkTime(node host.Host, validators []peer.ID) {
	go func() {
		timeShift := getNetworkTimeShift(node, validators)
		shared.SetNetworkTimeShift(timeShift)
	}()
}
