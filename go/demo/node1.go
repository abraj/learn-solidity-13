package demo

import (
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
)

func Node1() {
	privateKeyStr := "3077020101042008074adae939fc135ba0eed6a153115eca6315b3497e5d3f1597de5b31605e58a00a06082a8648ce3d030107a14403420004b21a4ce7c28b24708b44db8f6e4d5680270828d755f9d1d5f4325c30b079ba8a0daadc90c36abf3949a8752dc482f8dc6f205c8f46292c6b1454a24bbee7e847"
	privKey := utils.HexToPrivKey(privateKeyStr)
	privateKey := utils.ConvertECDSAKeyToLibp2p(privKey)

	// start a libp2p node
	node, err := libp2p.New(
		libp2p.Identity(privateKey),
		// libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/8001"),
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
