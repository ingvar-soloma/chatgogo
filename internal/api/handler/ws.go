package handler

import (
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
	"net/http"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Дозволяє з'єднання з будь-якого домену. У продакшені налаштувати!
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ServeWebSocket оновлює HTTP-з'єднання до WebSocket
func (h *Handler) ServeWebSocket(c *gin.Context) {
	// 1. Отримати AnonID з JWT (опускаємо логіку перевірки токена)
	anonID := c.Query("anon_id") // У реальному коді: перевірка JWT

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to upgrade connection"})
		return
	}

	// 2. Створення нового клієнта
	client := &chathub.Client{
		AnonID: anonID,
		Send:   make(chan models.ChatMessage, 256), // Буфер для повідомлень
		// ... зберігання самого conn
	}

	// 3. Реєстрація клієнта в Chat Hub
	h.Hub.RegisterCh <- client

	// 4. Запуск окремих Goroutines для читання/запису
	go client.ReadPump(conn, h.Hub) // Створюємо окрему Goroutine для читання
	go client.WritePump(conn)       // Створюємо окрему Goroutine для запису

	// NOTE: Функції ReadPump та WritePump повинні обробляти нескінченний цикл
	// читання/запису та відправляти дані в Hub.IncomingCh.
}
