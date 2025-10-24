package storage

import (
	"chatgogo/backend/internal/models"

	"github.com/go-redis/redis/v8"
)

// SaveUser зберігає користувача в PostgreSQL
func (s *Service) SaveUser(user *models.User) error {
	return s.DB.Save(user).Error
}

// SaveRoom зберігає кімнату в PostgreSQL
func (s *Service) SaveRoom(room *models.ChatRoom) error {
	return s.DB.Save(room).Error
}

// IsUserBanned перевіряє статус бану в Redis (швидка перевірка)
func (s *Service) IsUserBanned(anonID string) (bool, error) {
	// Приклад: перевіряємо, чи існує ключ у Redis для бану
	key := "ban:" + anonID
	status, err := s.Redis.Get(s.Ctx, key).Result()

	// Якщо помилка (не знайдено), повертаємо false
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// Якщо знайдено і статус "shadow" або "active"
	return status != "", nil
}

// PublishMessage публікує повідомлення в Redis Pub/Sub
func (s *Service) PublishMessage(roomID string, msg models.ChatMessage) error {
	// Тут потрібно серіалізувати msg, наприклад, у JSON
	msgJSON := "..." // У реальному коді: json.Marshal(msg)
	return s.Redis.Publish(s.Ctx, roomID, msgJSON).Err()
}
