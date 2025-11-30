package models

import (
	"github.com/google/uuid"
	"github.com/lib/pq" // Необхідний для pq.StringArray
	"gorm.io/gorm"
)

// User представляє користувача в системі.
// Містить інформацію про ідентифікацію, демографічні дані та інтереси.
type User struct {
	ID          string         `gorm:"primaryKey" json:"id"` // Анонімний UUID
	TelegramID  string         `gorm:"uniqueIndex"`          // Може бути nil
	Age         int            // Вік користувача
	Gender      string         // Стать користувача
	Interests   pq.StringArray `gorm:"type:text[]"` // Для зберігання тегів
	RatingScore int            // Оцінка співрозмовника
}

// BeforeCreate — це хук GORM, який викликається перед створенням запису.
// Він генерує новий UUID для користувача, якщо ID ще не встановлено.
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return
}
