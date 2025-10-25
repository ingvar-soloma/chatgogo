package storage

import (
	"chatgogo/backend/internal/models"
	"context"
	"encoding/json"
	"errors"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Storage interface {
	SaveUser(user *models.User) error
	SaveRoom(room *models.ChatRoom) error
	IsUserBanned(anonID string) (bool, error)
	// PublishMessage Cache Operations
	PublishMessage(roomID string, msg models.ChatMessage) error
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
	// В ідеалі, тут має бути підписка на ВСІ активні RoomID.
	// АЛЕ для простоти тестування, можна використовувати шаблон (якщо ви його налаштували)
	// АБО: Ваша система повинна динамічно підписуватися на нові канали.

	// ТИПОВА ПОМИЛКА: Якщо цей метод повертає заглушку або підписується на неіснуючий канал.

	// Якщо ви використовуєте ідею, де клієнти самі підписуються через Manager,
	// тоді проблема тут.
	// Якщо ж Listener підписується на всі канали, має бути:
	return s.Redis.Subscribe(context.Background(), "*") // Якщо ви використовуєте шаблони
}
