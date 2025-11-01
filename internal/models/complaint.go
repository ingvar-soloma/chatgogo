package models

import "gorm.io/gorm"

type Complaint struct {
	// gorm.Model включає поля ID, CreatedAt, UpdatedAt, DeletedAt
	gorm.Model

	RoomID         string `gorm:"type:uuid;not null;index"` // Ідентифікатор кімнати, де сталася скарга
	ReporterID     string `gorm:"type:text;not null"`       // AnonID користувача, який подав скаргу
	SuspectID      string `gorm:"type:text;not null;index"` // AnonID користувача, на якого подали скаргу
	LoggedMessages string `gorm:"type:text"`                // Зберігати як JSON або текст
	// Альтернативний підхід, вимагає, щоб GORM коректно обробляв тип JSONB
	//LoggedMessages []models.ChatMessage `gorm:"type:jsonb"`
	Reason string `gorm:"type:text"`             // Детальний опис скарги
	Status string `gorm:"type:text;default:new"` // Статус скарги: 'new', 'under_review', 'resolved', 'rejected'

}
