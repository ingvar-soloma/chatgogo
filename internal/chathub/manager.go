package chathub

import (
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"log"
)

// Client представляє одне активне з'єднання
type Client struct {
	AnonID string
	RoomID string
	// Send - канал, через який ми надсилаємо дані клієнту (з'єднання)
	Send chan models.ChatMessage
	// WebSocket/TG-з'єднання (тут опускаємо деталі)
}

// ManagerService - Головний хаб, працює в одній Goroutine
type ManagerService struct {
	Clients map[string]*Client // Мапа активних клієнтів (AnonID -> Client)

	// Channels для вхідних даних
	IncomingCh     chan models.ChatMessage   // Вхідні повідомлення від клієнтів
	MatchRequestCh chan models.SearchRequest // Запити на пошук співрозмовника
	RegisterCh     chan *Client              // Реєстрація нового з'єднання
	UnregisterCh   chan *Client              // Відключення

	Storage storage.Storage
}

// NewManagerService створює та ініціалізує сервіс
func NewManagerService(s storage.Storage) *ManagerService {
	return &ManagerService{
		Clients:        make(map[string]*Client),
		IncomingCh:     make(chan models.ChatMessage),
		MatchRequestCh: make(chan models.SearchRequest),
		RegisterCh:     make(chan *Client),
		UnregisterCh:   make(chan *Client),
		Storage:        s,
	}
}

// Run запускає основну Goroutine хаба
func (m *ManagerService) Run() {
	// Вся бізнес-логіка відбувається тут, у нескінченному циклі select
	for {
		select {
		case client := <-m.RegisterCh:
			m.Clients[client.AnonID] = client // Додаємо клієнта
			log.Printf("Client registered: %s", client.AnonID)

		case client := <-m.UnregisterCh:
			if _, ok := m.Clients[client.AnonID]; ok {
				delete(m.Clients, client.AnonID) // Видаляємо клієнта
				// Тут логіка розриву кімнати, якщо необхідно
				log.Printf("Client unregistered: %s", client.AnonID)
			}

		case msg := <-m.IncomingCh:
			// 1. Перевірка фільтрів (NLP)
			// 2. Збереження в БД (Storage.SaveMessage)
			// 3. Публікація через Redis (Storage.PublishMessage)
			log.Printf("Received message from %s for room %s", msg.SenderID, msg.RoomID)
			m.Storage.PublishMessage(msg.RoomID, msg)

		case req := <-m.MatchRequestCh:
			// Запуск логіки Matcher. У реальному коді це може бути окрема Goroutine
			log.Printf("Starting match search for %s", req.UserID)
			// ... (пошук співрозмовника, повернення RoomID у req.ResultCh)
		}
	}
}
