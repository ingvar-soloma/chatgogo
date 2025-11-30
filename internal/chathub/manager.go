// Package chathub provides the core real-time communication hub for the application.
// It manages client connections, message routing, and matchmaking.
package chathub

import (
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"log"
)

// ClientRestorer is a function type that defines a factory for creating a Client.
// It's used to restore a client's session, for example, on application restart.
type ClientRestorer func(userID string) (Client, error)

// ManagerService acts as a central hub for managing clients and chat rooms.
// It handles client registration, unregistration, message routing, and matchmaking requests.
type ManagerService struct {
	// Clients is a map of active clients, keyed by their user ID.
	Clients map[string]Client

	// IncomingCh is a channel for receiving all incoming messages from clients.
	IncomingCh chan models.ChatMessage
	// MatchRequestCh is a channel for queuing users who are looking for a chat partner.
	MatchRequestCh chan models.SearchRequest
	// RegisterCh is a channel for handling new client registrations.
	RegisterCh chan Client
	// UnregisterCh is a channel for handling client disconnections.
	UnregisterCh chan Client

	// Storage provides access to the data persistence layer.
	Storage storage.Storage
	// PubSubCh is a channel for receiving messages from the Redis Pub/Sub subscription.
	PubSubCh chan models.ChatMessage
	// ClientRestorer is a function used to recreate a client's state during session recovery.
	ClientRestorer ClientRestorer
}

// NewManagerService creates and returns a new ManagerService instance.
func NewManagerService(s storage.Storage) *ManagerService {
	return &ManagerService{
		Clients:        make(map[string]Client),
		IncomingCh:     make(chan models.ChatMessage, 10),
		MatchRequestCh: make(chan models.SearchRequest, 10),
		RegisterCh:     make(chan Client, 10),
		UnregisterCh:   make(chan Client, 10),
		Storage:        s,
		PubSubCh:       make(chan models.ChatMessage, 10),
	}
}

// Run starts the main event loop for the ManagerService.
// It listens on all its channels and processes incoming events, such as client
// registrations, messages, and matchmaking requests. This function is intended
// to be run as a goroutine.
func (m *ManagerService) Run() {
	log.Println("Chat Hub Manager started and listening to channels...")
	m.StartPubSubListener()
	m.RecoverActiveRooms()

	for {
		select {
		case client := <-m.RegisterCh:
			m.handleRegister(client)
		case client := <-m.UnregisterCh:
			m.handleUnregister(client)
		case message := <-m.IncomingCh:
			m.handleIncomingMessage(message)
		case message := <-m.PubSubCh:
			m.handlePubSubMessage(message)
		}
	}
}

// SetClientRestorer sets the function that will be used to restore client sessions.
func (m *ManagerService) SetClientRestorer(restorer ClientRestorer) {
	m.ClientRestorer = restorer
}

// RestoreClientSession recreates and registers a client session for a given user ID.
// This is useful for maintaining state across application restarts.
func (m *ManagerService) RestoreClientSession(userID string) error {
	if m.ClientRestorer == nil {
		return nil // No restorer configured
	}

	if _, ok := m.Clients[userID]; ok {
		return nil // Client already exists
	}

	client, err := m.ClientRestorer(userID)
	if err != nil {
		return err
	}

	m.RegisterCh <- client
	log.Printf("Restored client session for %s", userID)
	return nil
}

// RecoverActiveRooms loads active room information from the database on startup
// to restore the state of ongoing chats.
func (m *ManagerService) RecoverActiveRooms() {
	log.Println("Starting active room recovery process...")
	activeRoomIDs, err := m.Storage.GetActiveRoomIDs()
	if err != nil {
		log.Printf("ERROR: Failed to retrieve active rooms from storage: %v", err)
		return
	}

	for _, roomID := range activeRoomIDs {
		room, err := m.Storage.GetRoomByID(roomID)
		if err != nil {
			log.Printf("WARNING: Room %s not found in DB. Skipping.", roomID)
			continue
		}
		log.Printf("Restored active room %s between %s and %s.", roomID, room.User1ID, room.User2ID)
	}
	log.Printf("Recovery complete. Found %d previously active rooms.", len(activeRoomIDs))
}

func (m *ManagerService) handleRegister(client Client) {
	m.Clients[client.GetUserID()] = client
	log.Printf("Client registered: %s", client.GetUserID())
}

func (m *ManagerService) handleUnregister(client Client) {
	if _, ok := m.Clients[client.GetUserID()]; ok {
		delete(m.Clients, client.GetUserID())
		close(client.GetSendChannel())
		log.Printf("Client unregistered: %s", client.GetUserID())
	}
}

func (m *ManagerService) handleIncomingMessage(message models.ChatMessage) {
	if message.Type == "command_start" {
		m.MatchRequestCh <- models.SearchRequest{UserID: message.SenderID}
		return
	}

	if err := m.Storage.SaveMessage(&message); err != nil {
		log.Printf("ERROR: Failed to save message: %v", err)
		return
	}

	if err := m.Storage.PublishMessage(message.RoomID, message); err != nil {
		log.Printf("ERROR: Failed to publish message: %v", err)
	}
}

func (m *ManagerService) handlePubSubMessage(message models.ChatMessage) {
	room, err := m.Storage.GetRoomByID(message.RoomID)
	if err != nil {
		log.Printf("ERROR: Room not found for pub/sub message: %v", err)
		return
	}

	// Determine the recipient
	var recipientID string
	if message.SenderID == room.User1ID {
		recipientID = room.User2ID
	} else {
		recipientID = room.User1ID
	}

	if client, ok := m.Clients[recipientID]; ok {
		select {
		case client.GetSendChannel() <- message:
		default:
			log.Printf("WARN: Client send channel full, message dropped for user %s", recipientID)
		}
	}
}
