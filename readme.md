## libp2p

### How to run (js)

```bash
cd src

# first node
npx tsx index.ts

# second node
npx tsx index.ts /ip4/127.0.0.1/tcp/64139/p2p/first-node-multiaddr..
```

### How to run (`go/ping`, `go/dial`)

```bash
# update import in `main.go`
cd go

# first node
go run main.go

# second node
go run main.go /ip4/127.0.0.1/tcp/64139/p2p/first-node-multiaddr..
```

### How to run (`go/demo`)

```bash
cd go
go build -o libp2p-node

# first node
# go run main.go 1
./libp2p-node 1

# second node
# go run main.go 2
./libp2p-node 2

# third node
# go run main.go 3
./libp2p-node 3
```

### go-libp2p

- https://github.com/libp2p/go-libp2p
- https://github.com/libp2p/go-libp2p/blob/master/options.go
- https://www.youtube.com/watch?v=BUc4xta7Mfk
- https://ipfscluster.io/documentation/guides/consensus/

### Consensus

- https://www.youtube.com/playlist?list=PLZvgWu86XaWkpnQa6-OA7DG6ilM_RnxhW
- https://youtu.be/diqGSOLpyQ8?list=PLZvgWu86XaWkpnQa6-OA7DG6ilM_RnxhW&t=137
- [Verifiable Delay Functions - Dan Boneh](https://www.youtube.com/watch?v=dN-1q8c50q0)
- [BFT algorithms for blockchain](https://chatgpt.com/c/672d5b7f-1010-8006-bf78-898186505d51)
- https://arxiv.org/pdf/1807.04938
- https://github.com/tendermint/tendermint/blob/main/spec/README.md
- https://docs.tendermint.com/v0.34/introduction/what-is-tendermint.html
- https://docs.cometbft.com/v0.38/
- [Types of consensus among oracle nodes](https://chatgpt.com/c/670e903e-35fc-8006-9c87-d94ab96a1366)
- https://www.youtube.com/watch?v=yCMj_dfeO0k

### Chains

- https://safe.global/
- https://cosmos.network/
- https://tutorials.cosmos.network/academy/1-what-is-cosmos/

### Protocols

- https://developer.litprotocol.com/user-wallets/pkps/quick-start
- https://fluence.dev/docs/learn/fluence-comparison

### js-libp2p

- https://libp2p.github.io/js-libp2p/
- https://docs.libp2p.io/guides/getting-started/
- https://github.com/libp2p/js-libp2p/blob/main/doc/GETTING_STARTED.md
- https://github.com/libp2p/js-libp2p/blob/main/doc/CONFIGURATION.md
- https://github.com/libp2p/js-libp2p/blob/main/doc/SERVICES.md
- https://github.com/libp2p/js-libp2p-examples
- https://github.com/libp2p/js-libp2p-examples/tree/main/examples/js-libp2p-example-peer-and-content-routing
- https://github.com/libp2p/js-libp2p-examples/tree/main/examples/js-libp2p-example-chat
