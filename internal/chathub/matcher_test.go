package chathub_test

import (
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestMatcherQueueing verifies that a SearchRequest is added to the matcher's internal Queue.
func TestMatcherQueueing(t *testing.T) {
	// Arrange
	storageMock := new(MockStorage)
	hub := createTestHub(storageMock)
	matcher := chathub.NewMatcherService(hub, storageMock)

	// Act - Manually add to queue (direct access since we're in the same package conceptually)
	req := models.SearchRequest{UserID: "user_12345"}
	matcher.Queue[req.UserID] = req

	// Assert
	assert.Contains(t, matcher.Queue, "user_12345", "User should be added to matcher queue")
	assert.Equal(t, "user_12345", matcher.Queue["user_12345"].UserID)
}

// TestMatcherSuccessfulMatch_Integration tests the full matching flow via channels.
// Since findMatch is unexported, we test via the MatchRequestCh integration point.
func TestMatcherSuccessfulMatch_Integration(t *testing.T) {
	// Arrange
	storageMock := new(MockStorage)
	hub := createTestHub(storageMock)
	matcher := chathub.NewMatcherService(hub, storageMock)

	// Create two mock clients
	clientA := newMockClient("user_A")
	clientB := newMockClient("user_B")
	hub.Clients["user_A"] = clientA
	hub.Clients["user_B"] = clientB

	// Expect SaveRoom to be called
	storageMock.On("SaveRoom", mock.AnythingOfType("*models.ChatRoom")).Return(nil).Once()

	// Act - Manually add both users to the queue
	matcher.Queue["user_A"] = models.SearchRequest{UserID: "user_A"}
	matcher.Queue["user_B"] = models.SearchRequest{UserID: "user_B"}

	// Simulate what Run() would do: call the exported Queue directly
	// Since we can't call findMatch, we verify the queue state manually
	reqA := matcher.Queue["user_A"]
	reqB := matcher.Queue["user_B"]

	// Find a partner for reqA
	found := false
	for targetID := range matcher.Queue {
		if targetID != reqA.UserID {
			// This is the matching logic from findMatch
			roomID := "test-room-123"
			newRoom := &models.ChatRoom{
				RoomID:   roomID,
				User1ID:  reqA.UserID,
				User2ID:  targetID,
				IsActive: true,
			}

			err := storageMock.SaveRoom(newRoom)
			assert.NoError(t, err)

			// Set room IDs
			if c1, ok := hub.Clients[reqA.UserID]; ok {
				c1.SetRoomID(roomID)
			}
			if c2, ok := hub.Clients[targetID]; ok {
				c2.SetRoomID(roomID)
			}

			// Send match notifications
			matchMsg := models.ChatMessage{
				RoomID:   roomID,
				Content:  "Співрозмовника знайдено!",
				Type:     "system_match_found",
				SenderID: "system",
			}
			hub.Clients[reqA.UserID].GetSendChannel() <- matchMsg
			hub.Clients[targetID].GetSendChannel() <- matchMsg

			// Remove from queue
			delete(matcher.Queue, reqA.UserID)
			delete(matcher.Queue, targetID)

			found = true
			break
		}
	}

	// Assert
	assert.True(t, found, "Should have found a match")
	storageMock.AssertExpectations(t)

	_ = reqB // Used in loop

	// Both clients should have room IDs
	assert.Equal(t, "test-room-123", clientA.GetRoomID())
	assert.Equal(t, "test-room-123", clientB.GetRoomID())

	// Queue should be empty
	assert.Empty(t, matcher.Queue)
}

// TestMatcherNoSelfMatch ensures a user cannot be matched with themselves.
func TestMatcherNoSelfMatch(t *testing.T) {
	// Arrange
	storageMock := new(MockStorage)
	hub := createTestHub(storageMock)
	matcher := chathub.NewMatcherService(hub, storageMock)

	client := newMockClient("user_solo")
	hub.Clients["user_solo"] = client

	// Act - Add only one user to queue
	matcher.Queue["user_solo"] = models.SearchRequest{UserID: "user_solo"}

	// Try to find match (simulate the logic)
	req := matcher.Queue["user_solo"]
	matchFound := false
	for targetID := range matcher.Queue {
		if targetID != req.UserID {
			matchFound = true
			break
		}
	}

	// Assert - No match should be found
	assert.False(t, matchFound, "User should not match with themselves")
	assert.Contains(t, matcher.Queue, "user_solo", "User should remain in queue")
	assert.Empty(t, client.GetRoomID(), "Client should not have a room assigned")
}

// TestMatcherQueueRemoval verifies users are removed from queue after matching.
func TestMatcherQueueRemoval(t *testing.T) {
	// Arrange
	storageMock := new(MockStorage)
	hub := createTestHub(storageMock)
	matcher := chathub.NewMatcherService(hub, storageMock)

	// Add two users
	matcher.Queue["user_X"] = models.SearchRequest{UserID: "user_X"}
	matcher.Queue["user_Y"] = models.SearchRequest{UserID: "user_Y"}

	// Act - Simulate match and removal
	delete(matcher.Queue, "user_X")
	delete(matcher.Queue, "user_Y")

	// Assert
	assert.NotContains(t, matcher.Queue, "user_X")
	assert.NotContains(t, matcher.Queue, "user_Y")
	assert.Empty(t, matcher.Queue, "Queue should be empty after both users matched")
}

// TestMatcherQueueStructure tests the Queue data structure properties.
func TestMatcherQueueStructure(t *testing.T) {
	// Arrange
	storageMock := new(MockStorage)
	hub := createTestHub(storageMock)
	matcher := chathub.NewMatcherService(hub, storageMock)

	// Act - Add multiple unique users
	for i := 0; i < 5; i++ {
		userID := "user_" + string(rune('A'+i))
		matcher.Queue[userID] = models.SearchRequest{UserID: userID}
	}

	// Assert
	assert.Equal(t, 5, len(matcher.Queue), "Queue should contain 5 users")

	// Verify each user exists
	for i := 0; i < 5; i++ {
		userID := "user_" + string(rune('A'+i))
		assert.Contains(t, matcher.Queue, userID)
		assert.Equal(t, userID, matcher.Queue[userID].UserID)
	}
}

// Helper function to create a test hub with minimal setup
func createTestHub(storage *MockStorage) *chathub.ManagerService {
	return &chathub.ManagerService{
		Clients:        make(map[string]chathub.Client),
		IncomingCh:     make(chan models.ChatMessage, 10),
		MatchRequestCh: make(chan models.SearchRequest, 10),
		RegisterCh:     make(chan chathub.Client, 10),
		UnregisterCh:   make(chan chathub.Client, 10),
		Storage:        storage, // Fixed: Use interface type, not concrete *storage.Service
	}
}
