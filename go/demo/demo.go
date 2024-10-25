package demo

import (
	"context"
	"fmt"
	"log"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// NOTE: The validator function is called while receiving as well as _sending_ messages
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
		for {
			msg, err := subs.Next(ctx)
			if err != nil {
				log.Println("Error receiving message:", err)
				return
			}

			fmt.Printf("Received message: %s from %s\n", msg.Data, msg.GetFrom())
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
