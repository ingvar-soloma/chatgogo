package models

import "time"

// ChatRoom represents a 1-on-1 chat session between two users.
// It holds the state of the chat, including participants and its active status.
type ChatRoom struct {
	// RoomID is the unique identifier for the chat room (UUID).
	RoomID string `gorm:"primaryKey"`
	// User1ID is the anonymous ID of the first user in the room.
	User1ID string
	// User2ID is the anonymous ID of the second user in the room.
	User2ID string
	// IsActive indicates whether the chat room is currently active.
	IsActive bool
	// StartedAt is the timestamp when the chat room was created.
	StartedAt time.Time
	// EndedAt is the timestamp when the chat room was closed.
	EndedAt time.Time
}
