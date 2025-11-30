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

	// --- ВІДНОВЛЕННЯ ---
	m.RecoverActiveRooms()
	// --------------------------------

	log.Println("Chat Hub Manager started and listening to channels...")

	for {
		select {
		case client := <-m.RegisterCh:
			// Новий клієнт підключився (WebSocket/TG)
			m.Clients[client.GetAnonID()] = client
			log.Printf("Client registered: %s", client.GetAnonID())

			// !!! Перевірка на активну кімнату !!!
			// Отримуємо RoomID клієнта з БД. Якщо є, встановлюємо його.
			activeRoomID, err := m.Storage.GetActiveRoomIDForUser(client.GetAnonID())
			if err == nil && activeRoomID != "" {
				client.SetRoomID(activeRoomID)
				log.Printf("Client %s reconnected and restored to room %s.", client.GetAnonID(), activeRoomID)
				// Можна надіслати повідомлення про повторне підключення
				client.GetSendChannel() <- models.ChatMessage{
					Type:     "system_reconnect",
					SenderID: "system",
					RoomID:   activeRoomID,
					Content:  "🎉 Ви успішно відновили з'єднання з чатом!",
				}
			}

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
			case "command_search", "command_start":
				// Це команда на пошук співрозмовника. Надсилаємо в Matcher.
				log.Printf("Routing search command from %s to Matcher...", msg.SenderID)

				if client, ok := m.Clients[msg.SenderID]; ok && client.GetRoomID() != "" {
					log.Printf("Client %s is already in room %s. Ignoring search command.", msg.SenderID, client.GetRoomID())

					// Повідомляємо клієнта, що він вже в чаті
					client.GetSendChannel() <- models.ChatMessage{
						Type:     "system_info",
						SenderID: "system",
						Content:  "❌ Ви вже перебуваєте в активному чаті. Скористайтеся /stop, щоб завершити поточний чат.",
					}
					continue // Ігноруємо подальшу обробку пошуку
				}

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
						Type:     "system_search_start",
						SenderID: "system",
						Content:  "🔍 *Пошук співрозмовника розпочато...* Очікуйте з'єднання.",
						// RoomID тут порожній, оскільки кімнати ще немає
					}
					select {
					case client.GetSendChannel() <- searchStartMessage:
						// OK
					default:
						log.Printf("WARNING: Client %s send channel full during search start.", client.GetAnonID())
					}
				}

			case "command_next":
				log.Printf("Handling 'command_next' from %s", msg.SenderID)

				roomID := msg.RoomID // Отримуємо RoomID, який надіслав клієнт

				// 1. ЛОГІКА ЗАВЕРШЕННЯ ЧАТУ (аналогічно command_stop)
				if roomID != "" {
					// Закриваємо кімнату в БД
					if err := m.Storage.CloseRoom(roomID); err != nil {
						log.Printf("ERROR: Failed to close room %s during next command: %v", roomID, err)
					}

					// 1.1. Створюємо системне повідомлення про вихід для партнера
					partnerMessage := models.ChatMessage{
						Type:     "system_match_stop_partner",
						SenderID: "system",
						RoomID:   roomID,
						Content:  "Співрозмовник покинув чат та почав пошук нового.",
					}

					// 1.2. Публікуємо повідомлення для всіх інших серверів (якщо є)
					// Інші клієнти в цій кімнаті отримають це повідомлення
					m.Storage.PublishMessage(roomID, partnerMessage)

					// 1.3. Скидаємо RoomID ініціатора (це відбудеться і в tg_client, але для Hub робимо тут)
					if initiatorClient, ok := m.Clients[msg.SenderID]; ok {
						initiatorClient.SetRoomID("")
						// Повідомлення для ініціатора про успішне завершення
						initiatorClient.GetSendChannel() <- models.ChatMessage{
							Type:    "system_info",
							Content: "Чат завершено. 🔄 Починаємо пошук нового співрозмовника...",
						}
					}
				} else {
					// Якщо клієнт не був у чаті
					if client, ok := m.Clients[msg.SenderID]; ok {
						client.GetSendChannel() <- models.ChatMessage{
							Type:     "system_info",
							SenderID: "system",
							Content:  "Ви не були в активному чаті. 🔄 Починаємо пошук...",
						}
					}
				}

				// 2. ЛОГІКА ПОЧАТКУ ПОШУКУ (аналогічно command_start)
				// Створюємо запит на пошук і надсилаємо його Matcher'у
				request := models.SearchRequest{
					UserID: msg.SenderID,
					// ... інші параметри пошуку ...
				}
				m.MatchRequestCh <- request

			case "command_settings":
				log.Printf("Handling 'command_settings' from %s", msg.SenderID)

				if client, ok := m.Clients[msg.SenderID]; ok {
					// Надсилаємо клієнту системне повідомлення.
					// TgClient має перетворити це на Telegram-повідомлення з INLINE-кнопками.
					client.GetSendChannel() <- models.ChatMessage{
						Type:     "system_settings_menu", // Новий тип для Telegram
						SenderID: "system",
						Content:  "⚙️ Виберіть налаштування, які хочете змінити.",
						// У `Metadata` можна додати JSON із даними для формування inline-клавіатури
						Metadata: `{"buttons": [{"text": "Стать", "callback_data": "settings_gender"}, {"text": "Мова", "callback_data": "settings_lang"}]}`,
					}
				}

			case "command_report":
				log.Printf("Handling 'command_report' from %s", msg.SenderID)

				if client, ok := m.Clients[msg.SenderID]; ok {
					roomID := client.GetRoomID()

					// Перевіряємо, чи є активний чат
					if roomID == "" {
						client.GetSendChannel() <- models.ChatMessage{
							Type:     "system_info",
							SenderID: "system",
							Content:  "⚠️ Немає активного чату, щоб поскаржитися.",
						}
						continue
					}

					// 1. Створюємо об'єкт скарги
					complaint := &models.Complaint{
						RoomID:     roomID,
						ReporterID: msg.SenderID,
						Reason:     msg.Content, // Можливо, користувач вказав причину після /report
						Status:     "pending",
						// TODO: Завантажте історію повідомлень з Redis/DB та додайте її
					}

					// 2. Зберігаємо скаргу
					if err := m.Storage.SaveComplaint(complaint); err != nil {
						log.Printf("ERROR saving complaint for room %s: %v", roomID, err)
						client.GetSendChannel() <- models.ChatMessage{
							Type:     "system_error",
							SenderID: "system",
							Content:  "❌ Не вдалося зберегти скаргу. Спробуйте пізніше.",
						}
					} else {
						client.GetSendChannel() <- models.ChatMessage{
							Type:     "system_info",
							SenderID: "system",
							Content:  "✅ Дякуємо! Ваша скарга прийнята та буде розглянута модераторами.",
						}
					}
				}

			case "text", "photo", "sticker", "video", "voice", "animation", "video_note", "reply", "edit":
				// Це звичайне текстове повідомлення
				if msg.RoomID == "" {
					log.Printf("Message from %s rejected: No active room.", msg.SenderID)

					if client, ok := m.Clients[msg.SenderID]; ok {
						select {
						case client.GetSendChannel() <- models.ChatMessage{
							Type:    "system_info",
							Content: "❌ Ви не перебуваєте в активному чаті.",
						}:
						default:
						}
					}
					continue
				}

				// 1. Збереження в БД (Storage.SaveMessage)
				if err := m.Storage.SaveMessage(&msg); err != nil {
					log.Printf("ERROR: Failed to save message history for room %s: %v", msg.RoomID, err)
					// Надсилаємо системне повідомлення про помилку, якщо потрібно
					continue
				}

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

				// Закриваємо кімнату в БД
				if err := m.Storage.CloseRoom(roomID); err != nil {
					log.Printf("ERROR: Failed to close room %s during stop command: %v", roomID, err)
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
		}
	}
}
