package demo

import (
	"context"
	"libp2pdemo/utils"
	stdsync "sync"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	blockstore "github.com/ipfs/boxo/blockstore"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"

	// badger "github.com/ipfs/go-ds-badger3"
	crdt "github.com/ipfs/go-ds-crdt"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

func InitDatastore() *sync.MutexDatastore {
	// create a new in-memory datastore
	memoryDatastore := ds.NewMapDatastore()
	datastore := sync.MutexWrap(memoryDatastore)

	// // define the path for the persistent datastore
	// path := "./.data-cluster"

	// // ensure that the path exists
	// if err := os.MkdirAll(path, os.ModePerm); err != nil {
	// 	log.Fatalf("Failed to create datastore directory: %v", err)
	// }

	// // create a new Badger datastore
	// badgerDatastore, err := badger.NewDatastore(path, &badger.DefaultOptions)
	// datastore := sync.MutexWrap(badgerDatastore)
	// if err != nil {
	// 	log.Fatalf("Failed to create badger datastore: %v", err)
	// }

	return datastore
}

func InitDataCluster(ctx context.Context, node host.Host, datastore *sync.MutexDatastore, ps *pubsub.PubSub, kadDHT *dht.IpfsDHT) *crdt.Datastore {
	crdtKeyspace := ds.NewKey("/crdt/baadal")

	// create a blockstore with provided datastore as backend
	dsBlockstore := blockstore.NewBlockstore(datastore)

	// Use PubSub to create a CRDT broadcaster
	crdtTopic := "crdt-baadal"
	broadcaster, err := crdt.NewPubSubBroadcaster(ctx, ps, crdtTopic)
	if err != nil {
		panic(err)
	}

	// Create IPFS-Lite instance with blockstore and libp2p host
	ipfsLite, err := ipfslite.New(ctx, datastore, dsBlockstore, node, kadDHT, nil)
	if err != nil {
		panic(err)
	}
	dagService := ipfsLite.DAGService // Use IPFS-Lite's DAGService

	// Create the CRDT store
	crdtOptions := crdt.DefaultOptions()
	crdtOptions.RebroadcastInterval = 5 * time.Second

	store, err := crdt.New(datastore, crdtKeyspace, dagService, broadcaster, crdtOptions)
	if err != nil {
		panic(err)
	}

	return store
}

func InitNetworkTime(node host.Host, validators []peer.AddrInfo) int64 {
	// TODO: randomly select a smaller subset of validators for reduce communication overhead

	localTime := time.Now().UnixMilli()
	if len(validators) == 0 {
		return localTime
	}

	var (
		result = make(map[string]int64)
		mu     stdsync.Mutex
		wg     stdsync.WaitGroup
	)

	for _, validator := range validators {
		wg.Add(1) // Increment the WaitGroup counter

		go func(v peer.AddrInfo) {
			defer wg.Done() // Decrement the counter when the goroutine completes
			timestamp := queryTime(node, v.ID)

			mu.Lock() // maps are not thread-safe in Go
			result[v.ID.String()] = timestamp
			mu.Unlock()
		}(validator) // Pass `validator` as an argument to avoid closure issues
	}

	wg.Wait() // Wait for all goroutines to finish

	timestamps := []int64{}
	for _, value := range result {
		if value == 0 {
			continue
		}
		// TODO: filter extreme values
		// TODO: adjust for network delay
		timestamps = append(timestamps, value)
	}

	median := utils.Median(timestamps)
	return median
}
