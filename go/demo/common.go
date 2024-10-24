package demo

import (
	"fmt"
	"libp2pdemo/baadal/client"
	"libp2pdemo/baadal/request"
	"libp2pdemo/utils"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

func onConnected(node host.Host, peerID peer.ID) {
	protocolID := client.ProtocolID()

	resp := request.RequestService(node, peerID, protocolID, "clientinfo")
	m := utils.ResponseMap(resp)
	if m != nil && m["network_name"] == "baadal" && len(m["client_version"]) > 0 {
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
