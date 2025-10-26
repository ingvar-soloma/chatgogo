package chathub

import (
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"

	"github.com/gorilla/websocket"
)

// Client представляє одне активне WebSocket-з'єднання.

// ManagerService (додаємо поле для підписки Redis)
type ManagerService struct {
	Clients map[string]Client

	// Channels
	IncomingCh     chan models.ChatMessage
	MatchRequestCh chan models.SearchRequest

	RegisterCh   chan Client
	UnregisterCh chan Client

	Storage *storage.Service
	Conn    *websocket.Conn

	pubSubChannel chan models.ChatMessage
}

// NewManagerService (ініціалізація нового каналу)
func NewManagerService(s *storage.Service) *ManagerService {
	return &ManagerService{
		Clients:        make(map[string]Client),
		IncomingCh:     make(chan models.ChatMessage),
		MatchRequestCh: make(chan models.SearchRequest),
		RegisterCh:     make(chan Client),
		UnregisterCh:   make(chan Client),
		Storage:        s,
		pubSubChannel:  make(chan models.ChatMessage), // Ініціалізація
	}
}
