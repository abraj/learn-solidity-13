package demo

import (
	"context"
	"fmt"
	"libp2pdemo/utils"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	peerstore "github.com/libp2p/go-libp2p/core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
)

// CustomError defines a new error type
type CustomError struct {
	Message string
	Code    int
}

// Error method satisfies the error interface
func (e *CustomError) Error() string {
	return fmt.Sprintf("Error %d: %s", e.Code, e.Message)
}

func Node2(targetAddrStr string) {
	// start a libp2p node
	node, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
	)
	if err != nil {
		panic(err)
	}

	// print the node's PeerInfo in multiaddr format
	peerInfo := host.InfoFromHost(node)
	addrs, err := peerstore.AddrInfoToP2pAddrs(peerInfo)
	if err != nil {
		panic(err)
	}
	fmt.Println("libp2p node address:")
	for i := 0; i < len(addrs); i++ {
		fmt.Println("", addrs[i])
	}

	if len(os.Args) == 1 {
		err := &CustomError{
			Message: "No args provided!",
			Code:    500,
		}
		panic(err)
	}

	targetAddr, err := multiaddr.NewMultiaddr(targetAddrStr)
	if err != nil {
		panic(err)
	}
	targetAddrInfo, err := peerstore.AddrInfoFromP2pAddr(targetAddr)
	if err != nil {
		panic(err)
	}

	// Listen for peer connection events
	node.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			fmt.Printf("Connected to %s\n", conn.RemotePeer())
		},
		DisconnectedF: func(n network.Network, conn network.Conn) {
			fmt.Printf("Disconnected from %s\n", conn.RemotePeer())
		},
	})

	timer := utils.SetInterval(func() {
		peersList := node.Network().Peers()
		fmt.Printf("Number of peers: %d\n", len(peersList))
		for _, peer := range peersList {
			fmt.Println("", peer)
		}
	}, 8*time.Second)

	go func() {
		time.Sleep(11 * time.Second)

		// dial target node address
		if err := node.Connect(context.Background(), *targetAddrInfo); err != nil {
			panic(err)
		}
	}()

	// wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fmt.Println("Received signal, shutting down...")
	close(timer) // cleanup SetInterval timer

	// shut the node down
	if err := node.Close(); err != nil {
		panic(err)
	}
}
