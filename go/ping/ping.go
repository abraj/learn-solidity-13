package ping

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	multiaddr "github.com/multiformats/go-multiaddr"
)

func Start() {
	// start a libp2p node that listens on TCP port 2000 on the IPv4 loopback interface
	node, err := libp2p.New(
		// libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/2000"),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.Ping(false),
	)
	if err != nil {
		panic(err)
	}

	// print the node's listening addresses
	// fmt.Println("Node ID:", node.ID())
	// fmt.Println("Listen addresses:", node.Addrs())

	// print the node's PeerInfo in multiaddr format
	peerInfo := host.InfoFromHost(node)
	addrs, err := peer.AddrInfoToP2pAddrs(peerInfo)
	// peerInfo := peer.AddrInfo{
	// 	ID:    node.ID(),
	// 	Addrs: node.Addrs(),
	// }
	// addrs, err := peer.AddrInfoToP2pAddrs(&peerInfo)
	if err != nil {
		panic(err)
	}

	fmt.Println("libp2p node address:")
	for i := 0; i < len(addrs); i++ {
		fmt.Println("", addrs[i])
	}

	// configure our own ping protocol
	pingService := &ping.PingService{Host: node}
	node.SetStreamHandler(ping.ID, pingService.PingHandler)

	if len(os.Args) > 1 {
		addr, err := multiaddr.NewMultiaddr(os.Args[1])
		if err != nil {
			panic(err)
		}
		peerAddrInfo, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			panic(err)
		}
		if err := node.Connect(context.Background(), *peerAddrInfo); err != nil {
			panic(err)
		}

		fmt.Println("sending 5 ping messages to", addr)
		ch := pingService.Ping(context.Background(), peerAddrInfo.ID)
		for i := 0; i < 5; i++ {
			res := <-ch
			fmt.Println("ping response!", "RTT:", res.RTT)
		}
	} else {
		// wait for a SIGINT or SIGTERM signal
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		fmt.Println("Received signal, shutting down...")
	}

	// shut the node down
	if err := node.Close(); err != nil {
		panic(err)
	}
}
