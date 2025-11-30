package models

import "gorm.io/gorm"

// Complaint represents a user-submitted report against another user.
// It contains details about the complaint, including the chat room, participants,
// and the reason for the report.
type Complaint struct {
	// gorm.Model provides ID, CreatedAt, UpdatedAt, and DeletedAt fields.
	gorm.Model

	// RoomID is the identifier of the room where the reported incident occurred.
	RoomID string `gorm:"type:uuid;not null;index"`
	// ReporterID is the anonymous ID of the user who filed the complaint.
	ReporterID string `gorm:"type:text;not null"`
	// SuspectID is the anonymous ID of the user being reported.
	SuspectID string `gorm:"type:text;not null;index"`
	// LoggedMessages contains a log of the chat messages relevant to the complaint.
	// This is typically stored as a JSON string.
	LoggedMessages string `gorm:"type:text"`
	// Reason provides a detailed description of the complaint.
	Reason string `gorm:"type:text"`
	// Status indicates the current state of the complaint (e.g., 'new', 'under_review').
	Status string `gorm:"type:text;default:new"`
}
