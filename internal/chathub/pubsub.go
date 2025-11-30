package chathub

import (
	"chatgogo/backend/internal/models"
	"context"
	"encoding/json"
	"log"
)

// StartPubSubListener starts a goroutine that listens for messages on Redis Pub/Sub channels.
// This allows for horizontal scaling, as messages published in one application instance
// can be received and processed by all other instances.
func (m *ManagerService) StartPubSubListener() {
	go func() {
		ctx := context.Background()
		pubsub := m.Storage.Redis.PSubscribe(ctx, "*")
		defer pubsub.Close()

		if _, err := pubsub.Receive(ctx); err != nil {
			log.Printf("FATAL ERROR: Failed to subscribe to Redis PubSub: %v", err)
			return
		}

		ch := pubsub.Channel()
		log.Println("Redis PubSub listener started, listening to all channels (*).")

		for msg := range ch {
			var chatMsg models.ChatMessage
			if err := json.Unmarshal([]byte(msg.Payload), &chatMsg); err != nil {
				log.Printf("ERROR: Failed to unmarshal Redis message payload: %v | Payload: %s", err, msg.Payload)
				continue
			}
			m.pubSubChannel <- chatMsg
		}
	}()
}
