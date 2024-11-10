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
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
)

func Node1() {
	ecdsaPrivateKeyStr := "3077020101042008074adae939fc135ba0eed6a153115eca6315b3497e5d3f1597de5b31605e58a00a06082a8648ce3d030107a14403420004b21a4ce7c28b24708b44db8f6e4d5680270828d755f9d1d5f4325c30b079ba8a0daadc90c36abf3949a8752dc482f8dc6f205c8f46292c6b1454a24bbee7e847"
	privateKey := utils.DecodeECDSAPrivateKey(ecdsaPrivateKeyStr)
	// privateKey := utils.DecodeEd25519PrivateKey(privateKeyStr)

	bootstrapPeerAddrs := []string{
		// addresses of "other" bootstrap peers
	}

	ctx := context.Background()

	// ------------------

	// start a libp2p node
	node, err := libp2p.New(
		libp2p.Identity(privateKey),
		// libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/8001"),
		// libp2p.ListenAddrStrings("/ip4/172.232.108.85/tcp/8001"),
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

	// create a new PubSub service using the GossipSub router
	pubsubOpts := []pubsub.Option{
		pubsub.WithMessageSignaturePolicy(pubsub.StrictSign),
	}
	ps, err := pubsub.NewGossipSub(ctx, node, pubsubOpts...)
	if err != nil {
		log.Fatalf("Failed to create GossipSub: %v", err)
	}

	clientService := client.NewClientService()
	node.SetStreamHandler(client.ID, clientService.StreamHandler)

	fmt.Println("protocols:", node.Mux().Protocols())

	datastore := InitDatastore()
	// store := InitDataCluster(ctx, node, datastore, ps, kadDHT)

	AdjustNetworkTime(node, true)

	InitRandaoLoop(node, ps, datastore)

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

	timer1 := utils.SetInterval(func() {
		// peersList := node.Peerstore().Peers()
		peersList := node.Network().Peers()
		fmt.Printf("Number of peers: %d\n", len(peersList))
		for _, peerId := range peersList {
			fmt.Println("", peerId)
		}
	}, 8*time.Second)

	timer2 := utils.ExpBackOff(func() {
		AdjustNetworkTime(node, false)
	}, 3*time.Second, 30*time.Second)

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

	// DemoTopicRead(ctx, ps)
	// DemoDatastore(ctx, datastore)
	// DemoDataCluster(ctx, store)

	// ------------------

	// wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fmt.Println("Received signal, shutting down...")

	// cleanup SetInterval timers
	close(timer1)
	close(timer2)

	// close datastore
	datastore.Close()
	// store.Close() // hangs!

	// close DHT service
	if err := kadDHT.Close(); err != nil {
		panic(err)
	}

	// shut the node down
	if err := node.Close(); err != nil {
		panic(err)
	}
}
