package client

import (
	"fmt"
	"libp2pdemo/shared"
	"libp2pdemo/utils"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const (
	ID = "/baadal/client/1.0.0"
)

type ClientService struct {
	ProtocolID protocol.ID
}

func ProtocolID() protocol.ID {
	return protocol.ID(ID)
}

// Creates a new `client` service.
func NewClientService() ClientService {
	protocolID := ProtocolID()
	return ClientService{ProtocolID: protocolID}
}

func (cs *ClientService) StreamHandler(s network.Stream) {
	// fmt.Println("----------")
	// defer func() {
	// 	fmt.Println("----------")
	// }()

	err := handleStream(s)
	if err != nil {
		// NOTE: `err == io.EOF` means unexpected end of input stream
		fmt.Println(err)
		s.Reset()
	} else {
		s.Close()
	}
}

func handleStream(s network.Stream) error {
	// conn := s.Conn()
	// fmt.Println("Remote peer:", conn.RemotePeer().String())

	// Read from the stream
	data, err := utils.ReadStream(s)
	if err != nil {
		fmt.Print("Error reading from stream: ")
		return err
	}
	// fmt.Printf("Received: %s\n", data)

	// create a response based on the request (input) data
	resp := createResponse(data)

	// Respond to the stream
	err = utils.WriteStream(s, resp)
	if err != nil {
		fmt.Print("Could not write to stream: ")
		return err
	}

	return nil
}

func createResponse(data string) string {
	timestamp1 := shared.NetworkTime() // record the time when the request was received
	resp := ""
	clientInfo := "network_name: baadal; client_version: 0.1.0"
	if data == "clientinfo" {
		resp = clientInfo
	} else if data == "timestamp" {
		timestamp2 := shared.NetworkTime() // record the time when the response was sent
		resp = fmt.Sprintf("%s; epoch_t1: %d; epoch_t2: %d", clientInfo, timestamp1, timestamp2)
	}
	return resp
}
