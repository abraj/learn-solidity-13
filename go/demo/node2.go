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

func Node2() {
	ecdsaPrivateKeyStr := "3077020101042025d12858318183eb454b12cff1e99cc1544168e59298161003018601f2f48ae7a00a06082a8648ce3d030107a144034200046678e783977224c45a46ba565f840b70a78cbf7854e9d13625c670ffa7a06804ae605ee09b02ab6632c8425e34d1f998e6baf288d07e2de91e512bad35eadd88"
	privateKey := utils.DecodeECDSAPrivateKey(ecdsaPrivateKeyStr)
	// privateKey := utils.DecodeEd25519PrivateKey(privateKeyStr)

	bootstrapPeerAddrs := []string{
		"/ip4/127.0.0.1/tcp/8001/p2p/QmaT8zFZp8mKg6dAqxp4wNc7P9dn2WK6imPA37yG8zWwpq",
		// "/ip4/172.232.108.85/tcp/8001/p2p/QmaT8zFZp8mKg6dAqxp4wNc7P9dn2WK6imPA37yG8zWwpq",

		// ipfs public/bootstrap nodes
		// "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		// "/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		// "/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		// "/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		// "/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	}

	// TODO: fetch validator set (registry) from blockchain core contract
	validators := []string{
		"QmaT8zFZp8mKg6dAqxp4wNc7P9dn2WK6imPA37yG8zWwpq",
		"QmXfjanvuRK2sGZDrKa388ZNHak5DNhkT1Pzibf4YLu5FR",
		// "QmQNYyFizjhEFG2Sd4irQS2QvVa7kAwh9mNcsEraSBG4KB",
	}

	ctx := context.Background()

	// ------------------

	// start a libp2p node
	node, err := libp2p.New(
		libp2p.Identity(privateKey),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/8002"),
		// libp2p.ListenAddrStrings("/ip4/172.235.29.4/tcp/8002"),
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

	// create validator nodes' peerIDs using their string IDs
	var validatorsList []peer.ID
	for _, idStr := range validators {
		peerID, err := peer.Decode(idStr)
		if err != nil {
			panic(err)
		}
		validatorsList = append(validatorsList, peerID)
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
	_, err = pubsub.NewGossipSub(ctx, node, pubsubOpts...)
	if err != nil {
		log.Fatalf("Failed to create GossipSub: %v", err)
	}

	clientService := client.NewClientService()
	node.SetStreamHandler(client.ID, clientService.StreamHandler)

	fmt.Println("protocols:", node.Mux().Protocols())

	datastore := InitDatastore()
	// store := InitDataCluster(ctx, node, datastore, ps, kadDHT)

	AdjustNetworkTime(node, validatorsList)

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
		AdjustNetworkTime(node, validatorsList)
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

	// DemoTopicWrite(ctx, ps)
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
