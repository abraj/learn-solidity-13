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

func Node3(targetAddrStr string) {
	privateKeyStr := "307702010104205838f732713b7c8ab9e981458bba18d1736d5cfc37801d65e04b96dea5dae9a6a00a06082a8648ce3d030107a14403420004fbf030abd86aa51fd0d39b07521545896afd442f11342b2d467db791b477f97f4ad0da239d9c73fee1048a9608c6166c41b27af2d397f5368efa141226a31a97"
	privKey := utils.HexToPrivKey(privateKeyStr)
	privateKey := utils.ConvertECDSAKeyToLibp2p(privKey)

	// start a libp2p node
	node, err := libp2p.New(
		libp2p.Identity(privateKey),
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
		err := &utils.CustomError{
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