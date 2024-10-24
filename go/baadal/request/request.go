package request

import (
	"context"
	"fmt"
	"libp2pdemo/utils"
	"log"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

func RequestService(node host.Host, peerID peer.ID, protocolID protocol.ID, message string) string {
	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// open a new stream for the protocol ID
	s, err := node.NewStream(ctx, peerID, protocolID)
	if err != nil {
		// possibly, protocol not supported by other node
		fmt.Println("Unable to open stream:", err)
		return ""
	}
	// close the stream when done
	defer s.Close()

	// Send data to the other node
	err = utils.WriteStream(s, message)
	if err != nil {
		log.Fatal("Could not write to stream:", err)
	}

	respCh := make(chan string, 1)

	go func() {
		// Read response from the other node
		data, err := utils.ReadStream(s)
		if err != nil {
			fmt.Println("Error reading from stream:", err)
			respCh <- ""
			return
		}

		respCh <- data
	}()

	resp := ""

	// NOTE: blocked here until resp channel is resolved or ctx is timed out
	select {
	case resp = <-respCh:
		// response available on channel
	case <-ctx.Done():
		fmt.Println("Timeout: No response received within 10 seconds")
	}

	return resp
}
