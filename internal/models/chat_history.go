package models

import "gorm.io/gorm"

// ChatHistory represents a saved chat message in the PostgreSQL database.
// The embedded gorm.Model provides ID, CreatedAt, UpdatedAt, and DeletedAt fields,
// which serve as the message ID and timestamps.
type ChatHistory struct {
	gorm.Model // Includes fields ID (primary key, uint), CreatedAt, UpdatedAt, DeletedAt

	// RoomID is the identifier of the chat room where the message was sent.
	RoomID string `gorm:"type:uuid;not null;index:idx_room_msg"`
	// SenderID is the anonymous ID of the user who sent the message.
	SenderID string `gorm:"type:text;not null;index:idx_room_msg"`
	// Content is the main content of the message (e.g., text, file ID).
	Content string `gorm:"type:text;not null"`
	// Type indicates the kind of message (e.g., "text", "photo", "typing").
	Type string `gorm:"type:text;not null"`
	// Metadata contains additional information, such as captions for media.
	Metadata string `gorm:"type:text"`
	// ReplyToMessageID is a reference to the ID of the message being replied to.
	ReplyToMessageID *uint `gorm:"index"`

	// TgMessageIDSender is the Telegram message ID for the original sender.
	TgMessageIDSender *uint `gorm:"index"`
	// TgMessageIDReceiver is the Telegram message ID for the message's recipient.
	TgMessageIDReceiver *uint `gorm:"index"`
}
