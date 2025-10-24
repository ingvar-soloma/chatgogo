package models

import (
	"github.com/lib/pq" // Необхідний для pq.StringArray
)

type User struct {
	ID          string `gorm:"primaryKey" json:"id"` // Анонімний UUID
	TelegramID  string `gorm:"uniqueIndex"`          // Може бути nil
	Age         int
	Gender      string
	Interests   pq.StringArray `gorm:"type:text[]"` // Для зберігання тегів
	RatingScore int            // Оцінка співрозмовника
}
