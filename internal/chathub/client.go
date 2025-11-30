package chathub

import "chatgogo/backend/internal/models"

// Client — це інтерфейс для будь-якого типу підключення (WebSocket, Telegram тощо).
type Client interface {
	GetUserID() string
	GetRoomID() string
	SetRoomID(string) // Потрібно для Matcher, щоб встановити кімнату

	// GetSendChannel Повертає канал, куди Hub надсилає повідомлення ЦЬОМУ клієнту
	GetSendChannel() chan<- models.ChatMessage

	// Run Запускає цикли читання/запису (pumps)
	Run()
	// Close Закриває з'єднання та канали
	Close()
}
