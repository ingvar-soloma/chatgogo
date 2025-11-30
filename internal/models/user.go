package models

import (
	"github.com/google/uuid"
	"github.com/lib/pq" // Required for pq.StringArray
	"gorm.io/gorm"
)

// User represents a user in the system.
// It contains identification information, demographic data, and interests.
type User struct {
	ID                  string         `gorm:"primaryKey" json:"id"` // Anonymous UUID
	TelegramID          int64          `gorm:"uniqueIndex"`          // Can be nil
	Age                 int            // User's age
	Gender              string         // User's gender
	Interests           pq.StringArray `gorm:"type:text[]"` // Used for storing tags/interests
	RatingScore         int            // Rating score given by chat partners
	DefaultMediaSpoiler bool           `gorm:"default:true"` // User preference: if true, media sent by this user will have spoiler flag by default
	Language            string         `gorm:"default:'en'"` // User's interface language
}

// BeforeCreate is a GORM hook that is called before a record is created.
// It generates a new UUID for the user if the ID is not already set.
// The tx parameter is the GORM database transaction, which is part of the hook's signature.
// It returns an error if any issues are encountered.
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return
}
