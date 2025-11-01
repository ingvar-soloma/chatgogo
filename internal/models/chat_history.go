package models

import "gorm.io/gorm"

// ChatHistory представляє збережене повідомлення чату в PostgreSQL.
// ID, CreatedAt, UpdatedAt (з gorm.Model) будуть використовуватися як MessageID та Timestamp.
type ChatHistory struct {
	gorm.Model // Включає поля ID (primary key, uint), CreatedAt, UpdatedAt, DeletedAt

	RoomID           string `gorm:"type:uuid;not null;index:idx_room_msg"` // Ідентифікатор кімнати
	SenderID         string `gorm:"type:text;not null;index:idx_room_msg"` // AnonID користувача
	Content          string `gorm:"type:text;not null"`                    // Вміст повідомлення
	Type             string `gorm:"type:text;not null"`                    // "text", "photo", "typing", "media_url"
	Metadata         string `gorm:"type:text"`                             // Додаткова інформація
	ReplyToMessageID *uint  `gorm:"index"`                                 // Посилання на ID повідомлення (з ChatHistory.ID), на яке робиться реплай

	// Telegram Message ID повідомлення для Оригінального Відправника (SenderID)
	TgMessageIDSender *uint `gorm:"index"`
	// Telegram Message ID повідомлення для Оригінального Одержувача (Partner ID)
	TgMessageIDReceiver *uint `gorm:"index"`
}
