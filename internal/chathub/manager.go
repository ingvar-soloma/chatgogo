package chathub

import (
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Налаштування для WebSocket
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Client представляє одне активне WebSocket-з'єднання.
type Client struct {
	AnonID string
	RoomID string

	// ЕКСПОРТУЄМО (Велика літера) для доступу з пакету handler
	Conn *websocket.Conn
	Hub  *ManagerService // ЕКСПОРТУЄМО

	Send chan models.ChatMessage
}

// ManagerService (додаємо поле для підписки Redis)
type ManagerService struct {
	Clients map[string]*Client

	// Channels
	IncomingCh     chan models.ChatMessage
	MatchRequestCh chan models.SearchRequest
	RegisterCh     chan *Client
	UnregisterCh   chan *Client

	Storage *storage.Service
	Conn    *websocket.Conn

	pubSubChannel chan models.ChatMessage
}

// NewManagerService (ініціалізація нового каналу)
func NewManagerService(s *storage.Service) *ManagerService {
	return &ManagerService{
		Clients:        make(map[string]*Client),
		IncomingCh:     make(chan models.ChatMessage),
		MatchRequestCh: make(chan models.SearchRequest),
		RegisterCh:     make(chan *Client),
		UnregisterCh:   make(chan *Client),
		Storage:        s,
		pubSubChannel:  make(chan models.ChatMessage), // Ініціалізація
	}
}
