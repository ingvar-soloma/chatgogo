package chathub_test

import (
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"

	"github.com/stretchr/testify/mock"
)

// MockStorage is a comprehensive mock implementation of the storage.Storage interface.
// It uses testify/mock to allow flexible expectation setting in tests.
type MockStorage struct {
	mock.Mock
}

// User operations
func (m *MockStorage) SaveUser(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockStorage) SaveUserIfNotExists(telegramID string) (*models.User, error) {
	args := m.Called(telegramID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockStorage) IsUserBanned(anonID string) (bool, error) {
	args := m.Called(anonID)
	return args.Bool(0), args.Error(1)
}

// Room operations
func (m *MockStorage) SaveRoom(room *models.ChatRoom) error {
	args := m.Called(room)
	return args.Error(0)
}

func (m *MockStorage) CloseRoom(roomID string) error {
	args := m.Called(roomID)
	return args.Error(0)
}

func (m *MockStorage) GetActiveRoomIDs() ([]string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockStorage) GetActiveRoomIDForUser(userID string) (string, error) {
	args := m.Called(userID)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) GetRoomByID(roomID string) (*models.ChatRoom, error) {
	args := m.Called(roomID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ChatRoom), args.Error(1)
}

// Message operations
func (m *MockStorage) PublishMessage(roomID string, msg models.ChatMessage) error {
	args := m.Called(roomID, msg)
	return args.Error(0)
}

func (m *MockStorage) SaveTgMessageID(historyID uint, anonID string, tgMsgID int) error {
	args := m.Called(historyID, anonID, tgMsgID)
	return args.Error(0)
}

func (m *MockStorage) FindPartnerTelegramIDForReply(originalHistoryID uint, currentRecipientAnonID string) (*int, error) {
	args := m.Called(originalHistoryID, currentRecipientAnonID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	result := args.Get(0).(int)
	return &result, args.Error(1)
}

func (m *MockStorage) FindOriginalHistoryIDByTgID(tgMsgID uint) (*uint, error) {
	args := m.Called(tgMsgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	result := args.Get(0).(uint)
	return &result, args.Error(1)
}

func (m *MockStorage) FindOriginalHistoryIDByTgIDMedia(tgMsgID uint) (*uint, error) {
	args := m.Called(tgMsgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	result := args.Get(0).(uint)
	return &result, args.Error(1)
}

func (m *MockStorage) FindHistoryByID(id uint) (*models.ChatHistory, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ChatHistory), args.Error(1)
}

// Complaint operations
func (m *MockStorage) SaveComplaint(complaint *models.Complaint) error {
	args := m.Called(complaint)
	return args.Error(0)
}

// Queue operations
func (m *MockStorage) AddUserToSearchQueue(userID string) error {
	args := m.Called(userID)
	return args.Error(0)
}

func (m *MockStorage) RemoveUserFromSearchQueue(userID string) error {
	args := m.Called(userID)
	return args.Error(0)
}

func (m *MockStorage) GetSearchingUsers() ([]string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// MockClient is a test double for the chathub.Client interface.
type MockClient struct {
	mock.Mock
	userID string
	roomID string
	send   chan models.ChatMessage
}

func newMockClient(id string) *MockClient {
	return &MockClient{
		userID: id,
		roomID: "",
		send:   make(chan models.ChatMessage, 10), // Buffered to prevent blocking in tests
	}
}

func (c *MockClient) GetUserID() string {
	return c.userID
}

func (c *MockClient) GetRoomID() string {
	return c.roomID
}

func (c *MockClient) SetRoomID(id string) {
	c.roomID = id
	c.Called(id) // Record the call for assertion
}

func (c *MockClient) GetSendChannel() chan<- models.ChatMessage {
	return c.send
}

func (c *MockClient) Run() {
	c.Called()
}

func (c *MockClient) Close() {
	c.Called()
	close(c.send)
}

// Helper method to drain messages from the send channel (for test cleanup)
func (c *MockClient) DrainMessages() []models.ChatMessage {
	var messages []models.ChatMessage
	for {
		select {
		case msg := <-c.send:
			messages = append(messages, msg)
		default:
			return messages
		}
	}
}

// MockManagerService provides a lightweight stub of ManagerService for isolated testing.
// Unlike the full ManagerService, this doesn't run background goroutines.
type MockManagerService struct {
	Clients        map[string]chathub.Client
	IncomingCh     chan models.ChatMessage
	MatchRequestCh chan models.SearchRequest
	RegisterCh     chan chathub.Client
	UnregisterCh   chan chathub.Client
}

func NewMockManagerService() *MockManagerService {
	return &MockManagerService{
		Clients:        make(map[string]chathub.Client),
		IncomingCh:     make(chan models.ChatMessage, 10),
		MatchRequestCh: make(chan models.SearchRequest, 10),
		RegisterCh:     make(chan chathub.Client, 10),
		UnregisterCh:   make(chan chathub.Client, 10),
	}
}
