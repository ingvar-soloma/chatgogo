package chathub_test

import (
	"chatgogo/backend/internal/models"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
)

type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) SaveUser(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockStorage) SaveUserIfNotExists(telegramID int64) (*models.User, error) {
	args := m.Called(telegramID)
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockStorage) IsUserBanned(anonID string) (bool, error) {
	args := m.Called(anonID)
	return args.Bool(0), args.Error(1)
}

func (m *MockStorage) SaveRoom(room *models.ChatRoom) error {
	args := m.Called(room)
	return args.Error(0)
}

func (m *MockStorage) CloseRoom(roomID string) error {
	args := m.Called(roomID)
	return args.Error(0)
}

func (m *MockStorage) GetActiveRoomIDForUser(userID string) (string, error) {
	args := m.Called(userID)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) GetActiveRoomIDs() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockStorage) GetRoomByID(roomID string) (*models.ChatRoom, error) {
	args := m.Called(roomID)
	return args.Get(0).(*models.ChatRoom), args.Error(1)
}

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
	return args.Get(0).(*int), args.Error(1)
}

func (m *MockStorage) FindOriginalHistoryIDByTgID(tgMsgID uint) (*uint, error) {
	args := m.Called(tgMsgID)
	return args.Get(0).(*uint), args.Error(1)
}

func (m *MockStorage) FindOriginalHistoryIDByTgIDMedia(tgMsgID uint) (*uint, error) {
	args := m.Called(tgMsgID)
	return args.Get(0).(*uint), args.Error(1)
}

func (m *MockStorage) FindHistoryByID(id uint) (*models.ChatHistory, error) {
	args := m.Called(id)
	return args.Get(0).(*models.ChatHistory), args.Error(1)
}

func (m *MockStorage) SaveComplaint(complaint *models.Complaint) error {
	args := m.Called(complaint)
	return args.Error(0)
}

func (m *MockStorage) GetComplaintByID(complaintID uint) (*models.Complaint, error) {
	args := m.Called(complaintID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Complaint), args.Error(1)
}

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
	return args.Get(0).([]string), args.Error(1)
}
func (m *MockStorage) SaveMessage(msg *models.ChatMessage) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockStorage) GetChatHistory(roomID string) ([]models.ChatHistory, error) {
	args := m.Called(roomID)
	return args.Get(0).([]models.ChatHistory), args.Error(1)
}
func (m *MockStorage) SubscribeToAllRooms() *redis.PubSub {
	args := m.Called()
	return args.Get(0).(*redis.PubSub)
}

func (m *MockStorage) UpdateUserMediaSpoiler(userID string, value bool) error {
	args := m.Called(userID, value)
	return args.Error(0)
}

func (m *MockStorage) GetUserByID(userID string) (*models.User, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockStorage) UpdateUser(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockStorage) UpdateUserReputation(userID string, change int) error {
	args := m.Called(userID, change)
	return args.Error(0)
}

func (m *MockStorage) GetComplaintsForUser(userID string, since time.Time) ([]models.Complaint, error) {
	args := m.Called(userID, since)
	return args.Get(0).([]models.Complaint), args.Error(1)
}

func (m *MockStorage) GetLastBanDate(userID string) (int64, error) {
	args := m.Called(userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockStorage) UpdateUserLanguage(telegramID int64, languageCode string) error {
	args := m.Called(telegramID, languageCode)
	return args.Error(0)
}

func (m *MockStorage) GetUserByTelegramID(telegramID int64) (*models.User, error) {
	args := m.Called(telegramID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}
