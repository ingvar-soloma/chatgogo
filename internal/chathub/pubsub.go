package chathub

import (
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"context"
	"encoding/json"
	"log"
)

// StartPubSubListener запускає Goroutine, яка слухає Redis Pub/Sub
func (m *ManagerService) StartPubSubListener() {
	go func() {
		ctx := context.Background()

		// Підписуємося на спеціальний канал, який використовується для широкомовлення
		// (у нашому випадку, ми можемо слухати всі канали, названі за RoomID)
		// Для спрощення, тут слухаємо один глобальний канал 'chat:broadcast'
		pubsub := m.Storage.(*storage.Service).Redis.Subscribe(ctx, "chat:broadcast")
		defer pubsub.Close()

		ch := pubsub.Channel()

		for msg := range ch {
			var chatMsg models.ChatMessage
			err := json.Unmarshal([]byte(msg.Payload), &chatMsg)
			if err != nil {
				log.Printf("Error unmarshalling Redis message: %v", err)
				continue
			}

			// Надсилаємо отримане повідомлення у головний канал обробки (ManagerService)
			m.pubSubChannel <- chatMsg
		}
	}()
}

// Оновлення ManagerService.Run() для обробки pubSubChannel
func (m *ManagerService) Run() {
	m.StartPubSubListener() // Запускаємо слухача Redis

	for {
		select {
		// ... (RegisterCh, UnregisterCh, IncomingCh - як було)

		case msg := <-m.pubSubChannel:
			// Повідомлення надійшло від іншого Go-сервера через Redis!
			// 1. Знаходимо цільових клієнтів у цій кімнаті
			// 2. Надсилаємо їм повідомлення через їхні Client.Send канали
			for _, client := range m.Clients {
				if client.RoomID == msg.RoomID {
					// Надсилаємо повідомлення у канал WritePump клієнта
					select {
					case client.Send <- msg:
					default:
						// Обробка блокування каналу (наприклад, якщо клієнт повільний)
						close(client.Send)
						m.UnregisterCh <- client
					}
				}
			}
		}
	}
}
