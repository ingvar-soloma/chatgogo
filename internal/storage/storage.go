package storage

import (
	"chatgogo/backend/internal/models"
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Storage interface {
	SaveUser(user *models.User) error
	SaveRoom(room *models.ChatRoom) error
	IsUserBanned(anonID string) (bool, error)
	PublishMessage(roomID string, msg models.ChatMessage) error
	SaveComplaint(complaint *models.Complaint) error

	SaveTgMessageID(historyID uint, anonID string, tgMsgID int) error
	FindPartnerTelegramIDForReply(originalHistoryID uint, currentRecipientAnonID string) (*int, error)
	FindOriginalHistoryIDByTgID(tgMsgID uint) (*uint, error)
}

type Service struct {
	DB    *gorm.DB
	Redis *redis.Client
	Ctx   context.Context
}

// NewStorageService Constructor
func NewStorageService(db *gorm.DB, rdb *redis.Client) *Service {
	return &Service{
		DB:    db,
		Redis: rdb,
		Ctx:   context.Background(),
	}
}

// SaveUser зберігає користувача в PostgreSQL
func (s *Service) SaveUser(user *models.User) error {
	return s.DB.Save(user).Error
}

// SaveRoom зберігає кімнату в PostgreSQL
func (s *Service) SaveRoom(room *models.ChatRoom) error {
	return s.DB.Save(room).Error
}

// IsUserBanned перевіряє статус бану в Redis
func (s *Service) IsUserBanned(anonID string) (bool, error) {
	key := "ban:" + anonID
	status, err := s.Redis.Get(s.Ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return status != "", nil
}

// PublishMessage публікує повідомлення в Redis Pub/Sub
func (s *Service) PublishMessage(roomID string, msg models.ChatMessage) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if err := s.Redis.Publish(s.Ctx, roomID, string(msgBytes)).Err(); err != nil {
		return err
	}

	return nil
}

func (s *Service) SubscribeToAllRooms() *redis.PubSub {
	return s.Redis.PSubscribe(s.Ctx, "*")
}

func (s *Service) SaveComplaint(complaint *models.Complaint) error {
	if complaint.Status == "" {
		complaint.Status = "new"
	}

	result := s.DB.Create(complaint)

	if result.Error != nil {
		log.Printf("ERROR: Failed to save complaint for room %s: %v", complaint.RoomID, result.Error)
		return result.Error
	}

	// Приклад логування успіху, якщо потрібно
	// log.Printf("SUCCESS: Complaint created with ID: %d", complaint.ID)

	return nil
}

// SaveMessage зберігає повідомлення в PostgreSQL та оновлює ChatMessage ID
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

	// Створення запису в БД. history.ID буде заповнено GORM.
	if err := s.DB.Create(&history).Error; err != nil {
		log.Printf("ERROR: Failed to save message for room %s: %v", msg.RoomID, err)
		return err
	}

	// Оновлюємо ID в оригінальній структурі ChatMessage, щоб його можна було опублікувати.
	msg.ID = history.ID

	return nil
}

// GetChatHistory отримує історію повідомлень для кімнати
func (s *Service) GetChatHistory(roomID string) ([]models.ChatHistory, error) {
	var history []models.ChatHistory
	// Завантажуємо історію, сортуючи за часом створення
	if err := s.DB.Where("room_id = ?", roomID).Order("created_at asc").Find(&history).Error; err != nil {
		// Якщо помилка - повертаємо nil і log
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return history, nil // Повертаємо пустий список, а не помилку, якщо кімнату не знайдено
		}
		log.Printf("ERROR: Failed to get chat history for room %s: %v", roomID, err)
		return nil, err
	}
	return history, nil
}

// SaveTgMessageID оновлює ChatHistory двома TG Message ID,
// отриманими після надсилання повідомлень обом учасникам.
func (s *Service) SaveTgMessageID(historyID uint, anonID string, tgMsgID int) error {
	var history models.ChatHistory
	if err := s.DB.First(&history, historyID).Error; err != nil {
		return err
	}

	// Визначаємо, яке поле оновлювати:
	// Якщо AnonID, що надіслав ID, є відправником оригінального ChatHistory.
	tgID := uint(tgMsgID)
	if history.SenderID == anonID {
		history.TgMessageIDSender = &tgID
	} else {
		// Якщо це ID одержувача
		history.TgMessageIDReceiver = &tgID
	}

	return s.DB.Save(&history).Error
}

func (s *Service) FindOriginalHistoryIDByTgID(tgMsgID uint) (*uint, error) {
	var history models.ChatHistory
	// Шукаємо, чи є tgMsgID в полі TgMessageID_Sender або TgMessageID_Receiver
	// Якщо воно знайдено, це означає, що користувач відповів на це повідомлення.
	err := s.DB.Where("tg_message_id_sender = ?", tgMsgID).
		Or("tg_message_id_receiver = ?", tgMsgID).
		First(&history).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // Не знайдено, не є реплаєм в анонімному чаті
	}
	if err != nil {
		return nil, err
	}

	// Повертаємо внутрішній ID (gorm.Model.ID)
	return &history.ID, nil
}

func (s *Service) FindPartnerTelegramIDForReply(originalHistoryID uint, currentRecipientAnonID string) (*int, error) {
	var history models.ChatHistory
	if err := s.DB.First(&history, originalHistoryID).Error; err != nil {
		return nil, err
	}

	// Якщо поточний одержувач (currentRecipientAnonID) був відправником оригінального повідомлення (SenderID),
	// то ми повинні відповісти на TgMessageIDSender.
	if history.SenderID == currentRecipientAnonID {
		if history.TgMessageIDSender == nil {
			return nil, nil
		}
		tgID := int(*history.TgMessageIDSender)
		return &tgID, nil
	}

	// Інакше, поточний одержувач був одержувачем оригінального повідомлення,
	// і ми повинні відповісти на TgMessageIDReceiver.
	if history.TgMessageIDReceiver == nil {
		return nil, nil
	}
	tgID := int(*history.TgMessageIDReceiver)
	return &tgID, nil
}
