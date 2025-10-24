package storage

import (
	"chatgogo/backend/internal/models"
	"context"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// Storage визначає контракт для всіх операцій з даними.
type Storage interface {
	SaveUser(user *models.User) error
	SaveRoom(room *models.ChatRoom) error
	IsUserBanned(anonID string) (bool, error)
	// PublishMessage Cache Operations
	PublishMessage(roomID string, msg models.ChatMessage) error
}

// Service реалізує інтерфейс Storage
type Service struct {
	DB    *gorm.DB // PostgreSQL
	Redis *redis.Client
	Ctx   context.Context
}

// NewStorageService ініціалізує з'єднання
func NewStorageService(db *gorm.DB, rdb *redis.Client) *Service {
	return &Service{
		DB:    db,
		Redis: rdb,
		Ctx:   context.Background(),
	}
}
