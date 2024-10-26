package demo

import (
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	// badger "github.com/ipfs/go-ds-badger3"
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
