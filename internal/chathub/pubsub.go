package chathub

import (
	"chatgogo/backend/internal/models"
	"context"
	"encoding/json"
	"log"
)

// internal/chathub/manager.go

// StartPubSubListener запускає Goroutine, яка слухає Redis Pub/Sub
func (m *ManagerService) StartPubSubListener() {
	go func() {
		ctx := context.Background()

		// Використовуємо PSubscribe для підписки на всі можливі RoomID (*).
		// m.Storage.Redis — це *redis.Client
		pubsub := m.Storage.Redis.PSubscribe(ctx, "*")
		defer pubsub.Close() // Вирішує помилку 'Close'

		// Перевірка на помилку підписки (для *redis.PubSub потрібно викликати Receive)
		if _, err := pubsub.Receive(ctx); err != nil { // Вирішує помилку 'Receive'
			log.Printf("FATAL ERROR: Failed to subscribe to Redis PubSub: %v", err)
			return
		}

		ch := pubsub.Channel() // Вирішує помилку 'Channel'
		log.Println("Redis PubSub listener started, listening to all channels (*).")

		for msg := range ch {
			var chatMsg models.ChatMessage

			// 1. Декодування JSON
			// msg.Payload — це поле з *redis.Message
			if err := json.Unmarshal([]byte(msg.Payload), &chatMsg); err != nil { // Вирішує помилку 'Payload'
				log.Printf("ERROR: Failed to unmarshal Redis message payload: %v | Payload: %s", err, msg.Payload)
				continue
			}

			// ... (3. РОЗСИЛКА КЛІЄНТАМ)
			for _, client := range m.Clients {
				if client.RoomID == msg.Channel {
					select {
					case client.Send <- chatMsg:
						// OK
					default:
						log.Printf("WARNING: Client %s send channel full. Closing connection.", client.AnonID)
						// Реалізація безпечного відключення
						// delete(m.Clients, client.AnonID)
						// close(client.Send)
					}
				}
			}
		}
	}()
}

// Run Оновлення ManagerService.Run() для обробки pubSubChannel
func (m *ManagerService) Run() {
	// 1. Запускаємо Goroutine, яка слухатиме Redis (для горизонтального масштабування)
	m.StartPubSubListener()

	log.Println("Chat Hub Manager started and listening to channels...")

	for {
		select {
		case client := <-m.RegisterCh:
			// Новий клієнт підключився (WebSocket/TG)
			m.Clients[client.AnonID] = client
			log.Printf("Client registered: %s", client.AnonID)

		case client := <-m.UnregisterCh:
			// Клієнт відключився
			if _, ok := m.Clients[client.AnonID]; ok {
				delete(m.Clients, client.AnonID)
				close(client.Send) // Закриваємо канал, щоб WritePump завершилася
				// ! Логіка розриву кімнати !
				log.Printf("Client unregistered: %s", client.AnonID)
			}

		case req := <-m.MatchRequestCh:
			// Запит на пошук співрозмовника
			log.Printf("Starting match search for %s", req.UserID)
			// ! Тут буде викликаний Matcher !

		case msg := <-m.IncomingCh:

			switch msg.Type {
			case "command_search":
				// Це команда на пошук співрозмовника. Надсилаємо в Matcher.
				log.Printf("Routing search command from %s to Matcher...", msg.SenderID)

				// Створюємо структуру SearchRequest
				request := models.SearchRequest{
					UserID: msg.SenderID,
					// Тут можна додати фільтри з msg.Content, якщо він містить JSON-налаштування
				}
				m.MatchRequestCh <- request

			case "text":
				// Це звичайне текстове повідомлення
				if msg.RoomID == "" {
					log.Printf("Message from %s rejected: No active room.", msg.SenderID)
					// Можна надіслати клієнту системне повідомлення про помилку
					continue
				}

				// 1. Збереження в БД (Storage.SaveMessage)
				// 2. Публікація через Redis
				m.Storage.PublishMessage(msg.RoomID, msg)

			default:
				log.Printf("Unknown message type received: %s from %s", msg.Type, msg.SenderID)
			}
			// Вхідне повідомлення від клієнта (через ReadPump)
		}
	}
}
