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
	CloseRoom(roomID string) error
	SaveComplaint(complaint *models.Complaint) error
	SaveTgMessageID(historyID uint, anonID string, tgMsgID int) error
	SaveUserIfNotExists(telegramID string) error

	PublishMessage(roomID string, msg models.ChatMessage) error

	IsUserBanned(anonID string) (bool, error)

	FindPartnerTelegramIDForReply(originalHistoryID uint, currentRecipientAnonID string) (*int, error)
	FindOriginalHistoryIDByTgID(tgMsgID uint) (*uint, error)
	FindOriginalHistoryIDByTgIDMedia(tgMsgID uint) (*uint, error)
	FindHistoryByID(id uint) (*models.ChatHistory, error)

	GetActiveRoomIDForUser(userID string) (string, error)
	GetActiveRoomIDs() ([]string, error)
	GetRoomByID(roomID string) (*models.ChatRoom, error)

	AddUserToSearchQueue(userID string) error
	RemoveUserFromSearchQueue(userID string) error
	GetSearchingUsers() ([]string, error)
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

// CloseRoom закриває кімнату, встановлюючи IsActive = false та EndedAt = time.Now()
func (s *Service) CloseRoom(roomID string) error {
	return s.DB.Model(&models.ChatRoom{}).
		Where("room_id = ?", roomID).
		Updates(map[string]interface{}{
			"is_active": false,
			"ended_at":  gorm.Expr("NOW()"),
		}).Error
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
		Last(&history).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // Не знайдено, не є реплаєм в анонімному чаті
	}
	if err != nil {
		return nil, err
	}

	// Повертаємо внутрішній ID (gorm.Model.ID)
	return &history.ID, nil
}

func (s *Service) FindOriginalHistoryIDByTgIDMedia(tgMsgID uint) (*uint, error) {
	// Складний запит, що використовує DISTINCT ON (PostgreSQL) для групування
	// за унікальним контентом (file_id) та вибору останнього (найновішого) запису.
	rawSQL := `
        SELECT id
        FROM (
            -- 1. Знаходимо найновіший запис (за created_at DESC) для кожного унікального Content (file_id)
            SELECT DISTINCT ON (content)
                id,
                created_at
            FROM 
                chat_histories
            WHERE 
                tg_message_id_sender = ? OR tg_message_id_receiver = ?
            ORDER BY 
                content, 
                created_at ASC -- ⬅️ Вибираємо найстаріший запис у кожній групі 'content'
        ) AS latest_groups
        ORDER BY 
            created_at DESC -- 2. Вибираємо АБСОЛЮТНО найновіший запис серед усіх цих груп
        LIMIT 1
    `

	var resultID uint

	// Використовуємо Raw та Scan. Передаємо tgMsgID двічі для двох плейсхолдерів '?'
	err := s.DB.Raw(rawSQL, tgMsgID, tgMsgID).Scan(&resultID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) || resultID == 0 {
		return nil, nil // Не знайдено
	}
	if err != nil {
		return nil, err
	}

	return &resultID, nil
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

// FindHistoryByID повертає повний запис ChatHistory за його внутрішнім ID (gorm.Model.ID).
func (s *Service) FindHistoryByID(id uint) (*models.ChatHistory, error) {
	var history models.ChatHistory

	// Використовуємо .First() для пошуку за первинним ключем (ID)
	err := s.DB.First(&history, id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // Якщо запис не знайдено, повертаємо nil без помилки
	}
	if err != nil {
		return nil, err // Помилка бази даних
	}

	return &history, nil
}

// GetActiveRoomIDs повертає список усіх RoomID, які є активними в даний момент.
func (s *Service) GetActiveRoomIDs() ([]string, error) {
	var roomIDs []string

	// Обираємо лише RoomID з усіх записів, де IsActive = true
	if err := s.DB.Model(&models.ChatRoom{}).
		Where("is_active = ?", true).
		Pluck("room_id", &roomIDs).Error; err != nil {

		log.Printf("ERROR: Failed to retrieve active RoomIDs: %v", err)
		return nil, err
	}
	return roomIDs, nil
}

// GetActiveRoomIDForUser знаходить активний RoomID, в якому бере участь даний користувач.
func (s *Service) GetActiveRoomIDForUser(userID string) (string, error) {
	var room models.ChatRoom

	// Шукаємо кімнату, де користувач є User1ID АБО User2ID, і вона активна.
	err := s.DB.Where("is_active = ?", true).
		Where("user1_id = ? OR user2_id = ?", userID, userID).
		First(&room).Error // First() вибирає один запис

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil // Користувач не перебуває в активній кімнаті
	}
	if err != nil {
		log.Printf("ERROR: Failed to find active room for user %s: %v", userID, err)
		return "", err
	}

	return room.RoomID, nil
}

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
func (s *Service) SaveUserIfNotExists(telegramID string) error {
	var user models.User

	// Створюємо запис, який буде використовуватися, якщо користувача не знайдено
	defaults := models.User{
		TelegramID: telegramID, // Встановлюємо Telegram Chat ID
		// Інші поля за замовчуванням
	}

	// 1. Шукаємо існуючого користувача по AnonID
	// Примітка: Оскільки AnonID є унікальним, ми шукаємо по ньому.
	result := s.DB.Where("telegram_id = ?", telegramID).FirstOrCreate(&user, defaults)

	if result.Error != nil {
		log.Printf("ERROR: Failed to save user %s on first contact: %v", telegramID, result.Error)
		return result.Error
	}

	if result.RowsAffected > 0 {
		// Користувач був створений.
		log.Printf("INFO: New user %s saved to database (AnonID: %s).", user.ID, telegramID)
	}

	return nil
}

// AddUserToSearchQueue додає користувача до черги пошуку в Redis
func (s *Service) AddUserToSearchQueue(userID string) error {
	return s.Redis.SAdd(s.Ctx, "search_queue", userID).Err()
}

// RemoveUserFromSearchQueue видаляє користувача з черги пошуку в Redis
func (s *Service) RemoveUserFromSearchQueue(userID string) error {
	return s.Redis.SRem(s.Ctx, "search_queue", userID).Err()
}

// GetSearchingUsers повертає всіх користувачів, які зараз шукають пару
func (s *Service) GetSearchingUsers() ([]string, error) {
	return s.Redis.SMembers(s.Ctx, "search_queue").Result()
}
