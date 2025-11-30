package chathub

import (
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"log"

	"github.com/gorilla/websocket"
)

// Client представляє одне активне WebSocket-з'єднання.

type ClientRestorer func(userID string) (Client, error)

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

	pubSubChannel  chan models.ChatMessage
	ClientRestorer ClientRestorer
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

func (m *ManagerService) SetClientRestorer(restorer ClientRestorer) {
	m.ClientRestorer = restorer
}

func (m *ManagerService) RestoreClientSession(userID string) error {
	if m.ClientRestorer == nil {
		return nil // No restorer configured
	}

	// 1. Check if already exists
	if _, ok := m.Clients[userID]; ok {
		return nil
	}

	// 2. Create client using factory
	client, err := m.ClientRestorer(userID)
	if err != nil {
		return err
	}

	// 3. Register and Run
	m.Clients[userID] = client
	client.Run()
	log.Printf("Restored client session for %s", userID)
	return nil
}

// RecoverActiveRooms завантажує активні RoomID з Redis та оновлює стан Hub.
func (m *ManagerService) RecoverActiveRooms() {
	log.Println("Starting active room recovery process...")

	// 1. Отримуємо активні RoomID
	activeRoomIDs, err := m.Storage.GetActiveRoomIDs() // UNRESOLVED REFERENCE 1: Resolved via storage.Storage
	if err != nil {
		log.Printf("ERROR: Failed to retrieve active rooms from storage: %v", err)
		return
	}

	// 2. Ітеруємося по активних кімнатах
	for _, roomID := range activeRoomIDs {
		room, err := m.Storage.GetRoomByID(roomID) // UNRESOLVED REFERENCE 2: Resolved via storage.Storage
		if err != nil {
			log.Printf("WARNING: Room %s found in Redis but not in DB. Skipping.", roomID)
			continue
		}

		// 3. Доступ до публічних полів моделі
		log.Printf("Restored active room %s between %s and %s.", roomID, room.User1ID, room.User2ID)
		//                                                      ^ Resolved via public field
	}

	log.Printf("Recovery complete. Found %d previously active rooms.", len(activeRoomIDs))
}
