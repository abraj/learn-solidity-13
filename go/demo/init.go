package demo

import (
	"context"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	blockstore "github.com/ipfs/boxo/blockstore"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/ipfs/go-datastore/sync"

	// badger "github.com/ipfs/go-ds-badger3"
	crdt "github.com/ipfs/go-ds-crdt"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
)

func QueryWithPrefix(datastore ds.Datastore, prefix string) ([]ds.Key, []string, error) {
	// Prepare the query with a prefix filter
	q := query.Query{Prefix: prefix}

	// Run the query
	results, err := datastore.Query(context.Background(), q)
	if err != nil {
		return nil, nil, err
	}
	defer results.Close()

	// Collect keys and values
	var keys []ds.Key
	var values []string
	for result := range results.Next() {
		if result.Error != nil {
			return nil, nil, result.Error
		}
		keys = append(keys, ds.NewKey(result.Key))
		values = append(values, string(result.Value))
	}
	return keys, values, nil
}

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
