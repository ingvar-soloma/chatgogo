// Package storage provides the data persistence layer for the application,
// abstracting PostgreSQL and Redis operations.
package storage

import (
	"chatgogo/backend/internal/models"
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Storage defines the interface for all data persistence operations.
// It abstracts the underlying database and cache implementations.
type Storage interface {
	// User operations
	SaveUser(user *models.User) error
	UpdateUser(user *models.User) error
	UpdateUserReputation(userID string, change int) error
	GetComplaintsForUser(userID string, since time.Time) ([]models.Complaint, error)
	GetLastBanDate(userID string) (int64, error)
	SaveUserIfNotExists(telegramID int64) (*models.User, error)
	GetUserByTelegramID(telegramID int64) (*models.User, error)
	IsUserBanned(anonID string) (bool, error)
	UpdateUserMediaSpoiler(userID string, value bool) error

	// Room operations
	SaveRoom(room *models.ChatRoom) error
	CloseRoom(roomID string) error
	GetActiveRoomIDForUser(userID string) (string, error)
	GetActiveRoomIDs() ([]string, error)
	GetRoomByID(roomID string) (*models.ChatRoom, error)
	GetUserByID(userID string) (*models.User, error)

	// Message and History operations
	PublishMessage(roomID string, msg models.ChatMessage) error
	SaveMessage(msg *models.ChatMessage) error
	GetChatHistory(roomID string) ([]models.ChatHistory, error)
	SaveTgMessageID(historyID uint, anonID string, tgMsgID int) error
	FindPartnerTelegramIDForReply(originalHistoryID uint, currentRecipientAnonID string) (*int, error)
	FindOriginalHistoryIDByTgID(tgMsgID uint) (*uint, error)
	FindOriginalHistoryIDByTgIDMedia(tgMsgID uint) (*uint, error)
	FindHistoryByID(id uint) (*models.ChatHistory, error)

	// Complaint operations
	SaveComplaint(complaint *models.Complaint) error
	GetComplaintByID(complaintID uint) (*models.Complaint, error)

	// Search Queue operations
	AddUserToSearchQueue(userID string) error
	RemoveUserFromSearchQueue(userID string) error
	GetSearchingUsers() ([]string, error)
	SubscribeToAllRooms() *redis.PubSub

	// User settings
	UpdateUserLanguage(telegramID int64, languageCode string) error
}

// Service provides the implementation of the Storage interface,
// using a GORM DB client for PostgreSQL and a go-redis client for Redis.
type Service struct {
	DB    *gorm.DB
	Redis *redis.Client
	Ctx   context.Context
}

// NewStorageService creates and returns a new Service instance.
// It requires a GORM DB client and a Redis client as parameters.
func NewStorageService(db *gorm.DB, rdb *redis.Client) Storage {
	return &Service{
		DB:    db,
		Redis: rdb,
		Ctx:   context.Background(),
	}
}

// SaveUser saves a user record to the PostgreSQL database.
func (s *Service) SaveUser(user *models.User) error {
	return s.DB.Save(user).Error
}

// UpdateUser updates a user record in the PostgreSQL database.
func (s *Service) UpdateUser(user *models.User) error {
	return s.DB.Save(user).Error
}

// UpdateUserReputation updates a user's reputation score by a given amount, ensuring it stays between 0 and 1000.
func (s *Service) UpdateUserReputation(userID string, change int) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}

		newScore := user.ReputationScore + change
		if newScore > 1000 {
			newScore = 1000
		} else if newScore < 0 {
			newScore = 0
		}

		return tx.Model(&models.User{}).Where("id = ?", userID).Update("reputation_score", newScore).Error
	})
}

// GetComplaintsForUser retrieves all complaints against a user since a given time.
func (s *Service) GetComplaintsForUser(userID string, since time.Time) ([]models.Complaint, error) {
	var complaints []models.Complaint
	err := s.DB.Where("reported_user_id = ? AND created_at >= ?", userID, since).Find(&complaints).Error
	return complaints, err
}

// GetLastBanDate retrieves the last ban date for a user.
func (s *Service) GetLastBanDate(userID string) (int64, error) {
	var user models.User
	err := s.DB.Where("id = ?", userID).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return user.LastBanDate, nil
}

// SaveRoom saves a chat room record to the PostgreSQL database.
func (s *Service) SaveRoom(room *models.ChatRoom) error {
	return s.DB.Save(room).Error
}

// CloseRoom marks a chat room as inactive and sets its end time.
func (s *Service) CloseRoom(roomID string) error {
	return s.DB.Model(&models.ChatRoom{}).
		Where("room_id = ?", roomID).
		Updates(map[string]interface{}{
			"is_active": false,
			"ended_at":  gorm.Expr("NOW()"),
		}).Error
}

// IsUserBanned checks if a user is currently banned by looking up their ID in Redis.
func (s *Service) IsUserBanned(anonID string) (bool, error) {
	key := "ban:" + anonID
	_, err := s.Redis.Get(s.Ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil // Not banned if the key doesn't exist.
	}
	if err != nil {
		return false, err // An actual error occurred.
	}
	return true, nil // Banned if the key exists.
}

// PublishMessage serializes a ChatMessage to JSON and publishes it to a Redis Pub/Sub channel.
// The channel name is the roomID, allowing subscribers to listen for messages in specific rooms.
func (s *Service) PublishMessage(roomID string, msg models.ChatMessage) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.Redis.Publish(s.Ctx, roomID, string(msgBytes)).Err()
}

// SubscribeToAllRooms creates a Redis Pub/Sub subscription to all channels using a pattern.
// This is used by the hub to receive messages for all chat rooms.
func (s *Service) SubscribeToAllRooms() *redis.PubSub {
	return s.Redis.PSubscribe(s.Ctx, "*")
}

// SaveComplaint saves a user complaint record to the PostgreSQL database.
// It sets the default status to "new" if not provided.
func (s *Service) SaveComplaint(complaint *models.Complaint) error {
	if complaint.Status == "" {
		complaint.Status = "new"
	}

	result := s.DB.Create(complaint)
	if result.Error != nil {
		log.Printf("ERROR: Failed to save complaint for room %s: %v", complaint.RoomID, result.Error)
		return result.Error
	}
	return nil
}

// GetComplaintByID retrieves a complaint by its ID.
func (s *Service) GetComplaintByID(complaintID uint) (*models.Complaint, error) {
	var complaint models.Complaint
	err := s.DB.First(&complaint, complaintID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &complaint, nil
}

// SaveMessage persists a ChatMessage to the PostgreSQL database as a ChatHistory record.
// After saving, it updates the original ChatMessage's ID with the one generated by the database.
func (s *Service) SaveMessage(msg *models.ChatMessage) error {
	history := models.ChatHistory{
		RoomID:            msg.RoomID,
		SenderID:          msg.SenderID,
		Content:           msg.Content,
		Type:              msg.Type,
		Metadata:          msg.Metadata,
		ReplyToMessageID:  msg.ReplyToMessageID,
		TgMessageIDSender: msg.TgMessageIDSender,
	}

	// Create the record in the DB. GORM will populate history.ID.
	if err := s.DB.Create(&history).Error; err != nil {
		log.Printf("ERROR: Failed to save message for room %s: %v", msg.RoomID, err)
		return err
	}

	// Update the ID in the original ChatMessage struct for further use (e.g., publishing).
	msg.ID = history.ID
	return nil
}

// GetChatHistory retrieves the message history for a given room, ordered by creation time.
func (s *Service) GetChatHistory(roomID string) ([]models.ChatHistory, error) {
	var history []models.ChatHistory
	err := s.DB.Where("room_id = ?", roomID).Order("created_at asc").Find(&history).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return history, nil // Return an empty slice if no history is found.
		}
		log.Printf("ERROR: Failed to get chat history for room %s: %v", roomID, err)
		return nil, err
	}
	return history, nil
}

// SaveTgMessageID updates a ChatHistory record with the Telegram message ID.
// This is used to correlate internal message IDs with Telegram's IDs for replies and edits.
func (s *Service) SaveTgMessageID(historyID uint, anonID string, tgMsgID int) error {
	var history models.ChatHistory
	if err := s.DB.First(&history, historyID).Error; err != nil {
		return err
	}

	// Determine whether to update the sender's or receiver's message ID field.
	tgID := uint(tgMsgID)
	if history.SenderID == anonID {
		history.TgMessageIDSender = &tgID
	} else {
		history.TgMessageIDReceiver = &tgID
	}

	return s.DB.Save(&history).Error
}

// FindOriginalHistoryIDByTgID finds the internal message ID (ChatHistory.ID)
// corresponding to a given Telegram message ID. This is crucial for handling replies.
func (s *Service) FindOriginalHistoryIDByTgID(tgMsgID uint) (*uint, error) {
	var history models.ChatHistory
	// A message is a reply if its Telegram ID matches either the sender's or receiver's stored ID.
	err := s.DB.Where("tg_message_id_sender = ?", tgMsgID).
		Or("tg_message_id_receiver = ?", tgMsgID).
		Last(&history).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // Not a reply within the anonymous chat context.
	}
	if err != nil {
		return nil, err
	}
	return &history.ID, nil
}

// FindOriginalHistoryIDByTgIDMedia handles the complex case of identifying an original message
// when media is edited. It uses a DISTINCT ON query to find the earliest message
// with the same media content (file_id).
func (s *Service) FindOriginalHistoryIDByTgIDMedia(tgMsgID uint) (*uint, error) {
	rawSQL := `
        SELECT id
        FROM (
            -- 1. Find the earliest record (by created_at ASC) for each unique content (file_id).
            SELECT DISTINCT ON (content) id, created_at
            FROM chat_histories
            WHERE tg_message_id_sender = ? OR tg_message_id_receiver = ?
            ORDER BY content, created_at ASC
        ) AS latest_groups
        ORDER BY created_at DESC -- 2. Select the absolute latest record from these groups.
        LIMIT 1
    `
	var resultID uint
	err := s.DB.Raw(rawSQL, tgMsgID, tgMsgID).Scan(&resultID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || resultID == 0 {
		return nil, nil // Not found.
	}
	if err != nil {
		return nil, err
	}
	return &resultID, nil
}

// FindPartnerTelegramIDForReply determines the correct Telegram message ID to reply to.
// It looks up the original message and, based on who the current recipient is, returns
// the Telegram message ID of the other participant.
func (s *Service) FindPartnerTelegramIDForReply(originalHistoryID uint, currentRecipientAnonID string) (*int, error) {
	var history models.ChatHistory
	if err := s.DB.First(&history, originalHistoryID).Error; err != nil {
		return nil, err
	}

	// If the current recipient was the original sender, we need to reply to the sender's message.
	if history.SenderID == currentRecipientAnonID {
		if history.TgMessageIDSender != nil {
			tgID := int(*history.TgMessageIDSender)
			return &tgID, nil
		}
		return nil, nil
	}

	// Otherwise, the current recipient was the original receiver, so reply to the receiver's message.
	if history.TgMessageIDReceiver != nil {
		tgID := int(*history.TgMessageIDReceiver)
		return &tgID, nil
	}
	return nil, nil
}

// FindHistoryByID retrieves a complete ChatHistory record by its primary key.
func (s *Service) FindHistoryByID(id uint) (*models.ChatHistory, error) {
	var history models.ChatHistory
	err := s.DB.First(&history, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // Return nil without an error if the record is not found.
	}
	if err != nil {
		return nil, err
	}
	return &history, nil
}

// GetActiveRoomIDs returns a slice of all currently active room IDs.
func (s *Service) GetActiveRoomIDs() ([]string, error) {
	var roomIDs []string
	if err := s.DB.Model(&models.ChatRoom{}).
		Where("is_active = ?", true).
		Pluck("room_id", &roomIDs).Error; err != nil {
		log.Printf("ERROR: Failed to retrieve active RoomIDs: %v", err)
		return nil, err
	}
	return roomIDs, nil
}

// GetActiveRoomIDForUser finds the active room ID for a specific user.
// Returns an empty string if the user is not in an active room.
func (s *Service) GetActiveRoomIDForUser(userID string) (string, error) {
	var room models.ChatRoom
	err := s.DB.Where("is_active = ?", true).
		Where("user1_id = ? OR user2_id = ?", userID, userID).
		First(&room).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil // User is not in an active room.
	}
	if err != nil {
		log.Printf("ERROR: Failed to find active room for user %s: %v", userID, err)
		return "", err
	}
	return room.RoomID, nil
}

// GetRoomByID retrieves a chat room by its unique RoomID.
func (s *Service) GetRoomByID(roomID string) (*models.ChatRoom, error) {
	var room models.ChatRoom
	err := s.DB.Where("room_id = ?", roomID).First(&room).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("chat room not found")
	}
	if err != nil {
		log.Printf("ERROR: Failed to get room %s: %v", roomID, err)
		return nil, err
	}
	return &room, nil
}

// SaveUserIfNotExists finds a user by their Telegram ID or creates a new one if not found.
// It returns the found or newly created user.
func (s *Service) SaveUserIfNotExists(telegramID int64) (*models.User, error) {
	var user models.User
	defaults := models.User{
		TelegramID: telegramID,
	}

	result := s.DB.Where("telegram_id = ?", telegramID).FirstOrCreate(&user, defaults)
	if result.Error != nil {
		log.Printf("ERROR: Failed to save user %d on first contact: %v", telegramID, result.Error)
		return nil, result.Error
	}

	if result.RowsAffected > 0 {
		log.Printf("INFO: New user %s saved to database (TelegramID: %d).", user.ID, telegramID)
	}
	return &user, nil
}

// UpdateUserLanguage updates the user's language preference.
func (s *Service) UpdateUserLanguage(telegramID int64, languageCode string) error {
	return s.DB.Model(&models.User{}).
		Where("telegram_id = ?", telegramID).
		Update("language", languageCode).Error
}

// GetUserByTelegramID retrieves a user by their Telegram ID.
func (s *Service) GetUserByTelegramID(telegramID int64) (*models.User, error) {
	var user models.User
	if err := s.DB.Where("telegram_id = ?", telegramID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// AddUserToSearchQueue adds a user's ID to the Redis set representing the matchmaking queue.
func (s *Service) AddUserToSearchQueue(userID string) error {
	return s.Redis.SAdd(s.Ctx, "search_queue", userID).Err()
}

// RemoveUserFromSearchQueue removes a user's ID from the matchmaking queue in Redis.
func (s *Service) RemoveUserFromSearchQueue(userID string) error {
	return s.Redis.SRem(s.Ctx, "search_queue", userID).Err()
}

// GetSearchingUsers returns a slice of all user IDs currently in the matchmaking queue.
func (s *Service) GetSearchingUsers() ([]string, error) {
	return s.Redis.SMembers(s.Ctx, "search_queue").Result()
}

// UpdateUserMediaSpoiler updates the user's preference for default media spoiler flag.
func (s *Service) UpdateUserMediaSpoiler(userID string, value bool) error {
	return s.DB.Model(&models.User{}).
		Where("id = ?", userID).
		Update("default_media_spoiler", value).Error
}

// GetUserByID retrieves a user by their internal ID.
func (s *Service) GetUserByID(userID string) (*models.User, error) {
	var user models.User
	if err := s.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}
