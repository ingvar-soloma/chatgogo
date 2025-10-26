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

			log.Printf("Received message from Redis channel %s. Routing to clients.", msg.Channel)

			// ... (3. РОЗСИЛКА КЛІЄНТАМ)
			for _, client := range m.Clients {
				if client.GetRoomID() == msg.Channel {

					// Якщо кімната закривається, очищуємо RoomID на сервері
					if chatMsg.Type == "system_match_left" {
						client.SetRoomID("")
					}

					select {
					case client.GetSendChannel() <- chatMsg:
						// OK
					default:
						log.Printf("WARNING: Client %s send channel full. Closing connection.", client.GetAnonID())
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
			m.Clients[client.GetAnonID()] = client
			log.Printf("Client registered: %s", client.GetAnonID())

		case client := <-m.UnregisterCh:
			// Клієнт відключився
			if _, ok := m.Clients[client.GetAnonID()]; ok {
				delete(m.Clients, client.GetAnonID())
				client.Close() // Закриваємо канал, щоб WritePump завершилася
				// ! Логіка розриву кімнати !
				log.Printf("Client unregistered: %s", client.GetAnonID())
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

				// 1. Створюємо структуру SearchRequest
				request := models.SearchRequest{
					UserID: msg.SenderID,
					// Тут можна додати фільтри з msg.Content, якщо він містить JSON-налаштування
				}

				// 2. Надсилаємо запит у Matcher
				m.MatchRequestCh <- request

				// 3. Надсилаємо клієнту системне повідомлення про початок пошуку
				if client, ok := m.Clients[msg.SenderID]; ok {
					searchStartMessage := models.ChatMessage{
						Type:     "system_search_start", // ⬅️ НОВИЙ ТИП
						SenderID: "system",
						Content:  "🔍 **Пошук співрозмовника розпочато...** Очікуйте з'єднання.",
						// RoomID тут порожній, оскільки кімнати ще немає
					}
					select {
					case client.GetSendChannel() <- searchStartMessage:
						// OK
					default:
						log.Printf("WARNING: Client %s send channel full during search start.", client.GetAnonID())
					}
				}

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

			case "command_stop":
				log.Printf("Handling 'command_stop' from %s", msg.SenderID)

				// 1. ПЕРЕВІРКА КІМНАТИ
				roomID := msg.RoomID
				if roomID == "" {
					// Клієнт не в кімнаті.
					// TODO: Додайте логіку для видалення з черги Matcher'а, якщо потрібно.

					// Поки що просто повідомимо, що нічого зупиняти
					if client, ok := m.Clients[msg.SenderID]; ok {
						client.GetSendChannel() <- models.ChatMessage{
							Type:     "system_info", // Ми обробимо цей тип у tg_client
							SenderID: "system",
							Content:  "Ви не перебуваєте в активному чаті.",
						}
					}
					continue
				}

				// 2. СИСТЕМНІ ПОВІДОМЛЕННЯ (РІЗНІ ДЛЯ ІНІЦІАТОРА ТА ІНШИХ)

				// Повідомлення для ініціатора зупинки
				initiatorMessage := models.ChatMessage{
					Type:     "system_match_stop_self", // Новий тип для клієнта, що зупинив
					SenderID: "system",
					RoomID:   roomID,
					Content:  "Ви завершили чат.",
				}

				// Повідомлення для іншого учасника кімнати
				partnerMessage := models.ChatMessage{
					Type:     "system_match_stop_partner", // Новий тип для партнера
					SenderID: "system",
					RoomID:   roomID,
					Content:  "Співрозмовник покинув чат.",
				}

				// 3.1. Надсилаємо повідомлення ІНІЦІАТОРУ ЛОКАЛЬНО
				if initiatorClient, ok := m.Clients[msg.SenderID]; ok {
					select {
					case initiatorClient.GetSendChannel() <- initiatorMessage:
						// OK
					default:
						log.Printf("WARNING: Initiator client %s send channel full.", initiatorClient.GetAnonID())
						// Не вдалося надіслати.
					}
				}

				// 3.2. Публікуємо повідомлення для ПАРТНЕРА через Redis Pub/Sub
				// Це гарантує, що повідомлення отримає партнер, незалежно від того,
				// на якому Go-сервері він знаходиться.
				m.Storage.PublishMessage(roomID, partnerMessage)
			default:
				log.Printf("Unknown message type received: %s from %s", msg.Type, msg.SenderID)
			}
			// Вхідне повідомлення від клієнта (через ReadPump)
		}
	}
}
