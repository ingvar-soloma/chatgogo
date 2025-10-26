package chathub

import (
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"log"
	"time"

	"github.com/google/uuid"
)

// MatcherService відповідає за алгоритм пошуку співрозмовників.
type MatcherService struct {
	Hub     *ManagerService
	Storage storage.Storage

	// Queue - черга користувачів, які чекають на з'єднання
	// Використовуємо map для швидкого видалення та перевірки наявності.
	// Ключ: AnonID користувача, Значення: його SearchRequest
	Queue map[string]models.SearchRequest
}

// NewMatcherService створює новий Matcher.
func NewMatcherService(hub *ManagerService, s storage.Storage) *MatcherService {
	return &MatcherService{
		Hub:     hub,
		Storage: s,
		Queue:   make(map[string]models.SearchRequest),
	}
}

// Run запускає основну Goroutine Matcher'а.
func (m *MatcherService) Run() {
	log.Println("Matcher Service started.")

	// Головний цикл Matcher'а: слухає запити та намагається знайти збіги
	for {
		// 1. Очікування нового запиту на пошук
		select {
		case req := <-m.Hub.MatchRequestCh:
			m.Queue[req.UserID] = req // Додаємо новий запит у чергу
			log.Printf("New match request added to queue: %s", req.UserID)

			// 2. Спроба знайти пару
			m.findMatch(req)

		default:
			// Якщо немає нових запитів, але черга не пуста,
			// перевіряємо старі запити (для забезпечення з'єднання)
			if len(m.Queue) > 0 {
				for _, req := range m.Queue {
					m.findMatch(req)
				}
			}
			// Пауза, щоб не перевантажувати процесор при порожній черзі
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// findMatch намагається знайти співрозмовника для даного запиту (req)
func (m *MatcherService) findMatch(req models.SearchRequest) {
	// 1. Пошук потенційних кандидатів у черзі
	for targetID := range m.Queue {
		// Не шукати пару із самим собою
		if targetID == req.UserID {
			continue
		}

		// 2. Перевірка критеріїв збігу (Спрощена версія)

		// Примітка: У реальному коді тут буде перевірка віку, статі, тегів
		// і використання Storage/Cache для перевірки статусу бану та репутації.

		// Тут ми просто перевіряємо, чи є в черзі хтось інший, хто теж шукає.

		// 3. Якщо збіг знайдено:
		if true /* умова збігу */ {
			// 4. Створення кімнати
			roomID := uuid.New().String()
			newRoom := &models.ChatRoom{
				RoomID:    roomID,
				User1ID:   req.UserID,
				User2ID:   targetID,
				IsActive:  true,
				StartedAt: time.Now(),
			}

			if err := m.Storage.SaveRoom(newRoom); err != nil {
				log.Printf("Error saving new room: %v", err)
				return
			}

			// 5. Оновлення статусу клієнтів
			if client1, ok := m.Hub.Clients[req.UserID]; ok {
				client1.SetRoomID(roomID)
			}
			if client2, ok := m.Hub.Clients[targetID]; ok {
				client2.SetRoomID(roomID)
			}

			// 6. Повідомлення обох клієнтів про з'єднання
			matchMessage := models.ChatMessage{
				RoomID:   roomID,
				Content:  "Співрозмовника знайдено! Почніть діалог.",
				Type:     "system_match_found",
				SenderID: "system",
			}

			// Надсилаємо повідомлення обом клієнтам через їхні канали Send
			m.Hub.Clients[req.UserID].GetSendChannel() <- matchMessage
			m.Hub.Clients[targetID].GetSendChannel() <- matchMessage

			// 7. Видалення обох користувачів із черги
			delete(m.Queue, req.UserID)
			delete(m.Queue, targetID)

			log.Printf("Match found: %s and %s in room %s", req.UserID, targetID, roomID)
			return
		}
	}
}
