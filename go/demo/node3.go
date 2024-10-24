package demo

import (
	"context"
	"fmt"
	"libp2pdemo/baadal/client"
	"libp2pdemo/utils"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
)

func Node3() {
	privateKeyStr := "307702010104205838f732713b7c8ab9e981458bba18d1736d5cfc37801d65e04b96dea5dae9a6a00a06082a8648ce3d030107a14403420004fbf030abd86aa51fd0d39b07521545896afd442f11342b2d467db791b477f97f4ad0da239d9c73fee1048a9608c6166c41b27af2d397f5368efa141226a31a97"
	privKey := utils.HexToPrivKey(privateKeyStr)
	privateKey := utils.ConvertECDSAKeyToLibp2p(privKey)

	bootstrapPeerAddrs := []string{
		"/ip4/127.0.0.1/tcp/8002/p2p/QmXfjanvuRK2sGZDrKa388ZNHak5DNhkT1Pzibf4YLu5FR",
		// "/ip4/172.235.29.4/tcp/8002/p2p/QmXfjanvuRK2sGZDrKa388ZNHak5DNhkT1Pzibf4YLu5FR",
	}

	ctx := context.Background()

	// ------------------

	// start a libp2p node
	node, err := libp2p.New(
		libp2p.Identity(privateKey),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/8003"),
		// libp2p.ListenAddrStrings("/ip4/139.177.187.105/tcp/8003"),
	)
	if err != nil {
		panic(err)
	}

	// create bootstrap peer nodes' addrInfo using their string addresses
	var bootstrapPeers []peer.AddrInfo
	for _, addrStr := range bootstrapPeerAddrs {
		maddr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			panic(err)
		}
		addrInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			panic(err)
		}
		bootstrapPeers = append(bootstrapPeers, *addrInfo)
	}

	// create a new instance of DHT service
	dhtOpts := []dht.Option{
		dht.ProtocolPrefix("/baadal"),         // Custom DHT namespace
		dht.Mode(dht.ModeServer),              // Full DHT (server mode)
		dht.BootstrapPeers(bootstrapPeers...), // automatically set up the bootstrap peers
	}
	kadDHT, err := dht.New(ctx, node, dhtOpts...)
	if err != nil {
		log.Fatalf("Failed to create DHT: %v", err)
	}

	clientService := client.NewClientService()
	node.SetStreamHandler(client.ID, clientService.StreamHandler)

	fmt.Println("protocols:", node.Mux().Protocols())

	// ------------------

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

	// Listen for peer connection events
	node.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			peerID := conn.RemotePeer()
			fmt.Printf("Connected to %s\n", peerID)

			// "never block the callback"
			go func() {
				onConnected(node, peerID)
			}()
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

	// ------------------

	// // Connect to each bootstrap peer manually
	// // NOTE: Not needed if `dht.BootstrapPeers` option is configured during DHT initialization
	// for _, addrInfo := range bootstrapPeers {
	// 	// add target node address to peerstore as permanent address (bootstrap node)
	// 	node.Peerstore().AddAddrs(addrInfo.ID, addrInfo.Addrs, peerstore.PermanentAddrTTL)

	// 	// targetAddrInfo := addrInfo
	// 	targetAddrInfo := node.Peerstore().PeerInfo(addrInfo.ID)

	// 	// dial (target) peer node address
	// 	if err := node.Connect(ctx, targetAddrInfo); err != nil {
	// 		panic(err)
	// 	}
	// }

	// bootstrap the DHT instance
	// NOTE: will automatically connect to bootstrap peers if `dht.BootstrapPeers` option is configured during DHT initialization
	// NOTE: otherwise, call 'after' the node has connected to the configured bootstrap peers
	if err := kadDHT.Bootstrap(ctx); err != nil {
		log.Fatalf("Failed to bootstrap DHT: %v", err)
	}

	// ------------------

	// go func() {
	// 	time.Sleep(10 * time.Second)

	// 	peerID, err := peer.Decode("QmaT8zFZp8mKg6dAqxp4wNc7P9dn2WK6imPA37yG8zWwpq")
	// 	// peerID, err := peer.Decode("QmXfjanvuRK2sGZDrKa388ZNHak5DNhkT1Pzibf4YLu5FR")
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	fmt.Println(">>>>>>>>>>>>>> peerID:", peerID)

	// 	// Find the peer
	// 	addrInfo, err := kadDHT.FindPeer(ctx, peerID)
	// 	if err != nil {
	// 		log.Fatalf("Peer not found: (%s) %v", peerID, err)
	// 	}
	// 	fmt.Println(">> peerInfo:", addrInfo)
	// }()

	// ------------------

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
