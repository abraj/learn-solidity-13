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
	peer "github.com/libp2p/go-libp2p/core/peer"
	peerstore "github.com/libp2p/go-libp2p/core/peerstore"
	multiaddr "github.com/multiformats/go-multiaddr"
)

func Node3() {
	privateKeyStr := "307702010104205838f732713b7c8ab9e981458bba18d1736d5cfc37801d65e04b96dea5dae9a6a00a06082a8648ce3d030107a14403420004fbf030abd86aa51fd0d39b07521545896afd442f11342b2d467db791b477f97f4ad0da239d9c73fee1048a9608c6166c41b27af2d397f5368efa141226a31a97"
	privKey := utils.HexToPrivKey(privateKeyStr)
	privateKey := utils.ConvertECDSAKeyToLibp2p(privKey)

	// targetAddrStr := "/ip4/127.0.0.1/tcp/8002/p2p/QmXfjanvuRK2sGZDrKa388ZNHak5DNhkT1Pzibf4YLu5FR"
	targetAddrStr := "/ip4/172.235.29.4/tcp/8002/p2p/QmXfjanvuRK2sGZDrKa388ZNHak5DNhkT1Pzibf4YLu5FR"

	// start a libp2p node
	node, err := libp2p.New(
		libp2p.Identity(privateKey),
		// libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/8003"),
		libp2p.ListenAddrStrings("/ip4/139.177.187.105/tcp/8003"),
	)
	if err != nil {
		panic(err)
	}
	// fmt.Println("protocols:", node.Mux().Protocols())

	// print the node's PeerInfo in multiaddr format
	peerInfo := host.InfoFromHost(node)
	addrs, err := peer.AddrInfoToP2pAddrs(peerInfo)
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
	targetAddrInfo, err := peer.AddrInfoFromP2pAddr(targetAddr)
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
		// peersList := node.Peerstore().Peers()
		peersList := node.Network().Peers()
		fmt.Printf("Number of peers: %d\n", len(peersList))
		for _, peerId := range peersList {
			fmt.Println("", peerId)
		}
	}, 8*time.Second)

	go func() {
		time.Sleep(11 * time.Second)

		// add target node address to peerstore as permanent address (bootstrap node)
		node.Peerstore().AddAddrs(targetAddrInfo.ID, targetAddrInfo.Addrs, peerstore.PermanentAddrTTL)

		// addrInfo := *targetAddrInfo
		addrInfo := node.Peerstore().PeerInfo(targetAddrInfo.ID)

		// dial target node address
		if err := node.Connect(context.Background(), addrInfo); err != nil {
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
