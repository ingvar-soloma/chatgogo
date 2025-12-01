package chathub

import (
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"log"
	"time"

	"github.com/google/uuid"
)

// MatcherService is responsible for the matchmaking algorithm.
// It pairs users who are looking for a chat partner.
type MatcherService struct {
	// Hub is a reference to the central ManagerService.
	Hub *ManagerService
	// Storage provides access to the data persistence layer.
	Storage storage.Storage
	// Queue holds the users currently waiting to be matched.
	// A map is used for efficient lookups and deletions, with the user's ID as the key.
	Queue map[string]models.SearchRequest
}

// NewMatcherService creates and returns a new MatcherService instance.
func NewMatcherService(hub *ManagerService, s storage.Storage) *MatcherService {
	return &MatcherService{
		Hub:     hub,
		Storage: s,
		Queue:   make(map[string]models.SearchRequest),
	}
}

// Run starts the main goroutine for the MatcherService.
// It listens for new match requests and periodically attempts to find pairs.
func (m *MatcherService) Run() {
	log.Println("Matcher Service started.")
	m.restoreSearchQueue()

	// Main matcher loop: listens for requests and tries to find matches.
	for {
		select {
		case req := <-m.Hub.MatchRequestCh:
			m.AddUserToQueue(req)
			m.FindMatch(req)
		default:
			// If there are no new requests but the queue is not empty,
			// iterate over the queue to find matches.
			if len(m.Queue) > 1 {
				for _, req := range m.Queue {
					m.FindMatch(req)
				}
			}
			// Pause to prevent high CPU usage when the queue is empty or has one user.
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// restoreSearchQueue loads the list of searching users from storage on startup
// to restore the matchmaking queue's state.
func (m *MatcherService) restoreSearchQueue() {
	users, err := m.Storage.GetSearchingUsers()
	if err != nil {
		log.Printf("Error restoring search queue: %v", err)
		return
	}

	for _, userID := range users {
		if err := m.Hub.RestoreClientSession(userID); err != nil {
			log.Printf("Failed to restore session for %s: %v", userID, err)
			m.Storage.RemoveUserFromSearchQueue(userID)
			continue
		}
		m.Queue[userID] = models.SearchRequest{UserID: userID}
	}
	log.Printf("Restored %d users to search queue.", len(m.Queue))
}

// AddUserToQueue adds a new user to the matchmaking queue.
func (m *MatcherService) AddUserToQueue(req models.SearchRequest) {
	m.Queue[req.UserID] = req
	if err := m.Storage.AddUserToSearchQueue(req.UserID); err != nil {
		log.Printf("Error adding user to search queue in storage: %v", err)
	}
	log.Printf("New match request added to queue: %s", req.UserID)
}

// FindMatch attempts to find a chat partner for the given search request.
func (m *MatcherService) FindMatch(req models.SearchRequest) {
	// Collect all user IDs from the queue.
	userIDs := make([]string, 0, len(m.Queue))
	for id := range m.Queue {
		userIDs = append(userIDs, id)
	}

	// Fetch all users from the database in a single query.
	users, err := m.Storage.GetUsersByIDs(userIDs)
	if err != nil {
		log.Printf("Error getting users by IDs: %v", err)
		return
	}

	// Create a map for quick lookups.
	userMap := make(map[string]*models.User)
	for _, user := range users {
		userMap[user.ID] = user
	}

	user1 := userMap[req.UserID]

	// Iterate through the queue to find a potential match.
OuterLoop:
	for targetID := range m.Queue {
		if targetID == req.UserID {
			continue // Don't match a user with themselves.
		}

		user2 := userMap[targetID]

		// Check if user1 has blocked user2
		for _, blockedID := range user1.BlockedUsers {
			if blockedID == user2.ID {
				continue OuterLoop
			}
		}

		// Check if user2 has blocked user1
		for _, blockedID := range user2.BlockedUsers {
			if blockedID == user1.ID {
				continue OuterLoop
			}
		}

		m.createRoomForMatch(req.UserID, targetID)
		return
	}
}

// createRoomForMatch creates a new chat room for a pair of matched users.
func (m *MatcherService) createRoomForMatch(user1ID, user2ID string) {
	roomID := uuid.New().String()
	newRoom := &models.ChatRoom{
		RoomID:    roomID,
		User1ID:   user1ID,
		User2ID:   user2ID,
		IsActive:  true,
		StartedAt: time.Now(),
	}

	if err := m.Storage.SaveRoom(newRoom); err != nil {
		log.Printf("Error saving new room: %v", err)
		return
	}

	// Update the clients with the new room ID.
	if client1, ok := m.Hub.Clients[user1ID]; ok {
		client1.SetRoomID(roomID)
	}
	if client2, ok := m.Hub.Clients[user2ID]; ok {
		client2.SetRoomID(roomID)
	}

	// Notify both clients that a match has been found.
	matchMessage := models.ChatMessage{
		RoomID:   roomID,
		Content:  "system_match_found",
		Type:     "system_match_found",
		SenderID: "system",
	}
	m.Hub.Clients[user1ID].GetSendChannel() <- matchMessage
	m.Hub.Clients[user2ID].GetSendChannel() <- matchMessage

	// Remove both users from the queue.
	delete(m.Queue, user1ID)
	delete(m.Queue, user2ID)
	m.Storage.RemoveUserFromSearchQueue(user1ID)
	m.Storage.RemoveUserFromSearchQueue(user2ID)

	log.Printf("Match found: %s and %s in room %s", user1ID, user2ID, roomID)
}
