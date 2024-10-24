package demo

import (
	"fmt"
	"libp2pdemo/baadal/client"
	"libp2pdemo/baadal/request"
	"libp2pdemo/utils"

	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

func onConnected(node host.Host, peerID peer.ID) {
	protocolID := client.ProtocolID()

	resp := request.RequestService(node, peerID, protocolID, "clientinfo")
	fmt.Println("--------------")
	fmt.Println("resp:", resp)
	m := utils.ResponseMap(resp)
	fmt.Println("map:", m)
	if m != nil && m["network_name"] == "baadal" && len(m["client_version"]) > 0 {
		fmt.Println(">>", peerID, "Valid")
	} else {
		fmt.Println(">>", peerID, "Invalid")
	}
	fmt.Println("--------------")
}
