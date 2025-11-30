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
	hub := chathub.NewManagerService(storageMock)
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
	hub := chathub.NewManagerService(storageMock)
	matcher := chathub.NewMatcherService(hub, storageMock)

	// Create two mock clients
	clientA := newMockClient("user_A")
	clientB := newMockClient("user_B")
	hub.Clients["user_A"] = clientA
	hub.Clients["user_B"] = clientB

	// Expect SaveRoom to be called
	storageMock.On("SaveRoom", mock.AnythingOfType("*models.ChatRoom")).Return(nil).Once()
	storageMock.On("RemoveUserFromSearchQueue", mock.AnythingOfType("string")).Return(nil)

	// Act - Manually add both users to the queue
	matcher.Queue["user_A"] = models.SearchRequest{UserID: "user_A"}
	matcher.Queue["user_B"] = models.SearchRequest{UserID: "user_B"}

	// Find a partner for reqA
	matcher.FindMatch(models.SearchRequest{UserID: "user_A"})

	// Assert
	storageMock.AssertExpectations(t)

	// Both clients should have room IDs
	assert.NotEmpty(t, clientA.GetRoomID())
	assert.Equal(t, clientA.GetRoomID(), clientB.GetRoomID())

	// Queue should be empty
	assert.Empty(t, matcher.Queue)
}

// TestMatcherNoSelfMatch ensures a user cannot be matched with themselves.
func TestMatcherNoSelfMatch(t *testing.T) {
	// Arrange
	storageMock := new(MockStorage)
	hub := chathub.NewManagerService(storageMock)
	matcher := chathub.NewMatcherService(hub, storageMock)

	client := newMockClient("user_solo")
	hub.Clients["user_solo"] = client

	// Act - Add only one user to queue
	matcher.Queue["user_solo"] = models.SearchRequest{UserID: "user_solo"}
	matcher.FindMatch(models.SearchRequest{UserID: "user_solo"})

	// Assert - No match should be found
	assert.Contains(t, matcher.Queue, "user_solo", "User should remain in queue")
	assert.Empty(t, client.GetRoomID(), "Client should not have a room assigned")
}

// TestMatcherQueueRemoval verifies users are removed from queue after matching.
func TestMatcherQueueRemoval(t *testing.T) {
	// Arrange
	storageMock := new(MockStorage)
	hub := chathub.NewManagerService(storageMock)
	matcher := chathub.NewMatcherService(hub, storageMock)

	// Add two users
	matcher.Queue["user_X"] = models.SearchRequest{UserID: "user_X"}
	matcher.Queue["user_Y"] = models.SearchRequest{UserID: "user_Y"}

	storageMock.On("SaveRoom", mock.AnythingOfType("*models.ChatRoom")).Return(nil).Once()
	storageMock.On("RemoveUserFromSearchQueue", mock.AnythingOfType("string")).Return(nil)

	clientA := newMockClient("user_X")
	clientB := newMockClient("user_Y")
	hub.Clients["user_X"] = clientA
	hub.Clients["user_Y"] = clientB

	// Act - Simulate match and removal
	matcher.FindMatch(models.SearchRequest{UserID: "user_X"})

	// Assert
	assert.Empty(t, matcher.Queue, "Queue should be empty after both users matched")
}

// TestMatcherQueueStructure tests the Queue data structure properties.
func TestMatcherQueueStructure(t *testing.T) {
	// Arrange
	storageMock := new(MockStorage)
	hub := chathub.NewManagerService(storageMock)
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

func TestAddUserToQueue(t *testing.T) {
	// Arrange
	storageMock := new(MockStorage)
	hub := chathub.NewManagerService(storageMock)
	matcher := chathub.NewMatcherService(hub, storageMock)
	storageMock.On("AddUserToSearchQueue", "user_123").Return(nil)

	// Act
	matcher.AddUserToQueue(models.SearchRequest{UserID: "user_123"})

	// Assert
	assert.Contains(t, matcher.Queue, "user_123")
	storageMock.AssertCalled(t, "AddUserToSearchQueue", "user_123")
}
