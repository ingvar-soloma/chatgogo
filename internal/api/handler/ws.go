package handler

import (
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
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
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization token missing"})
		return
	}
	tokenString := authHeader[7:]

	// 2. Валідація та отримання AnonID з JWT
	anonID, err := h.validateAndGetAnonID(tokenString)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token or expired"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to upgrade connection"})
		return
	}

	// 1. Створення нового клієнта
	client := &chathub.WebSocketClient{
		Hub:    h.Hub, // Додано посилання на Hub
		UserID: anonID,
		Conn:   conn, // Збереження з'єднання
		Send:   make(chan models.ChatMessage, 256),
	}

	// 2. Реєстрація клієнта в Chat Hub
	h.Hub.RegisterCh <- client

	// 3. Запуск клієнта (це замінює старі виклики WritePump/ReadPump)
	// client.Run() сам запустить необхідні goroutines
	client.Run()
}
