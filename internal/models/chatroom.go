package models

import "time"

type ChatRoom struct {
	RoomID    string `gorm:"primaryKey"`
	User1ID   string
	User2ID   string
	IsActive  bool
	StartedAt time.Time
	EndedAt   time.Time
}
