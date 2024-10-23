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

### Docs

- https://libp2p.github.io/js-libp2p/
- https://docs.libp2p.io/guides/getting-started/
- https://github.com/libp2p/js-libp2p/blob/main/doc/GETTING_STARTED.md
- https://github.com/libp2p/js-libp2p/blob/main/doc/CONFIGURATION.md
- https://github.com/libp2p/js-libp2p/blob/main/doc/SERVICES.md
- https://github.com/libp2p/js-libp2p-examples
- https://github.com/libp2p/js-libp2p-examples/tree/main/examples/js-libp2p-example-peer-and-content-routing
- https://github.com/libp2p/js-libp2p-examples/tree/main/examples/js-libp2p-example-chat
