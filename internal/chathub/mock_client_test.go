package chathub_test

import (
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
)

type MockClient struct {
	userID      string
	roomID      string
	send        chan models.ChatMessage
	userType    string
	RecvChannel chan models.ChatMessage
}

func newMockClient(userID string) *MockClient {
	return &MockClient{
		userID:      userID,
		send:        make(chan models.ChatMessage, 10),
		RecvChannel: make(chan models.ChatMessage, 10),
	}
}

func (c *MockClient) GetUserID() string {
	return c.userID
}

func (c *MockClient) GetRoomID() string {
	return c.roomID
}

func (c *MockClient) SetRoomID(roomID string) {
	c.roomID = roomID
}

func (c *MockClient) GetSendChannel() chan<- models.ChatMessage {
	return c.RecvChannel
}

func (c *MockClient) GetUserType() string {
	return c.userType
}

func (c *MockClient) ReadPump(h *chathub.ManagerService) {
	// Not needed for testing
}

func (c *MockClient) WritePump() {
	// Not needed for testing
}

func (c *MockClient) Close() {
	// Not needed for testing
}

func (c *MockClient) Run() {
	// Not needed for testing
}
