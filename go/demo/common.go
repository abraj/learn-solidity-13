package demo

import (
	"fmt"
	"libp2pdemo/baadal/client"
	"libp2pdemo/baadal/request"
	"libp2pdemo/utils"
	"log"
	"strconv"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

func validateResponse(respMap map[string]string, expectedArgs ...string) bool {
	cond1 := respMap != nil && respMap["network_name"] == "baadal" && len(respMap["client_version"]) > 0
	if !cond1 {
		return false
	}

	if len(expectedArgs) > 0 {
		for _, val := range expectedArgs {
			item := respMap[val]
			cond := len(item) > 0
			if !cond {
				log.Printf("Expected arg (%s) not found in resp: %v", val, respMap)
				return false
			}
		}
	}

	return true
}

func onConnected(node host.Host, peerID peer.ID) {
	protocolID := client.ProtocolID()

	resp := request.RequestService(node, peerID, protocolID, "clientinfo")
	respMap := utils.ResponseMap(resp)
	if validateResponse(respMap) {
		// fmt.Println(">>", peerID, "Valid")
	} else {
		evictPeer(node, peerID)
		fmt.Println(">> Evicted:", peerID)
	}
}

func evictPeer(node host.Host, peerID peer.ID) {
	// kadDHT.RoutingTable().RemovePeer(peerId)
	// node.ConnManager().TagPeer(peerID, "blocked", -1000)

	node.Peerstore().ClearAddrs(peerID)
	node.Peerstore().RemovePeer(peerID)

	time.Sleep(2 * time.Second) // delay to avoid race condition among nodes
	node.Network().ClosePeer(peerID)
}

func queryTime(node host.Host, peerID peer.ID) int64 {
	protocolID := client.ProtocolID()

	resp := request.RequestService(node, peerID, protocolID, "timestamp")
	respMap := utils.ResponseMap(resp)
	if validateResponse(respMap, "epoch_timestamp") {
		epoch_timestamp := respMap["epoch_timestamp"]
		timestamp, err := strconv.ParseInt(epoch_timestamp, 10, 64)
		if err != nil {
			log.Println("Error converting string to int64:", epoch_timestamp, err)
			return 0
		}
		return timestamp
	}

	return 0
}
