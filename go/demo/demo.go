package demo

import (
	"context"
	"fmt"
	"libp2pdemo/utils"
	"log"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	crdt "github.com/ipfs/go-ds-crdt"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// NOTE: The validator function is called while receiving (non-local) messages
// as well as _sending_ messages.
func validatorPredicate(ctx context.Context, pid peer.ID, msg *pubsub.Message) bool {
	if len(msg.Data) == 0 {
		// empty message
		return false
	}
	if len(msg.Data) > 1024 {
		log.Println("[WARN] message size exceeds 1KB")
		return false
	}
	return true
}

func DemoTopicRead(ctx context.Context, ps *pubsub.PubSub) {
	// Set up topic subscription
	topicName := "welcome"

	err := ps.RegisterTopicValidator(topicName, validatorPredicate)
	if err != nil {
		panic(err)
	}
	topic, err := ps.Join(topicName)
	if err != nil {
		panic(err)
	}

	subs, err := topic.Subscribe()
	if err != nil {
		panic(err)
	}

	go func() {
		defer subs.Cancel()
		for {
			msg, err := subs.Next(ctx)
			if err != nil {
				log.Println("Error receiving message:", err)
				return
			}

			data := string(msg.Data)
			from := msg.GetFrom()

			fmt.Printf("Received message: %s from %s\n", data, from)
		}
	}()
}

func DemoTopicWrite(ctx context.Context, ps *pubsub.PubSub) {
	// Set up topic subscription
	topicName := "welcome"

	err := ps.RegisterTopicValidator(topicName, validatorPredicate)
	if err != nil {
		panic(err)
	}
	topic, err := ps.Join(topicName)
	if err != nil {
		panic(err)
	}

	go func() {
		time.Sleep(10 * time.Second)

		topic.Publish(ctx, []byte("")) // empty message
		topic.Publish(ctx, []byte("hello world!"))
	}()
}

func DemoDatastore(ctx context.Context, datastore *sync.MutexDatastore) {
	key := ds.NewKey("/example/key")
	value := []byte("Hello, Datastore!")

	// put a value in the datastore
	if err := datastore.Put(ctx, key, value); err != nil {
		log.Fatal(err)
	}

	// retrieve a value from the datastore
	storedValue, err := datastore.Get(ctx, key)
	if err != nil {
		// log.Fatal(err)
		panic(err)
	}
	fmt.Printf("Stored value: %s\n", string(storedValue))
}

func DemoDataCluster(ctx context.Context, store *crdt.Datastore) {
	key := ds.NewKey("/example/key")

	doStore := false

	go func() {
		time.Sleep(13 * time.Second)

		// Example: Put a key-value pair into the datastore
		if doStore {
			err := store.Put(ctx, key, []byte("hello world"))
			if err != nil {
				panic(err)
			}
		}
	}()

	go func() {
		time.Sleep(1 * time.Second)

		utils.SetInterval(func() {
			// Example: Get a value from the datastore
			value, err := store.Get(ctx, key)
			if err != nil {
				// panic(err)
				fmt.Println("Value: NOT found!")
				return
			}
			fmt.Println("Value:", string(value))
		}, 8*time.Second)
	}()
}
