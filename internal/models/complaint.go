package models

type Complaint struct {
	ComplaintID    string `gorm:"primaryKey"`
	ReporterID     string
	TargetID       string
	RoomID         string
	Reason         string
	LoggedMessages string // Зберігати як JSON або текст
	Status         string // "New", "Processed", "Banned"
}
