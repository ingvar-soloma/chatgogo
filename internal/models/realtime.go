package models

type ChatMessage struct {
	SenderID string `json:"sender_id"`
	RoomID   string `json:"room_id"`
	Content  string `json:"content"`
	Type     string `json:"type"`               // "text", "photo", "typing", "media_url"
	Metadata string `json:"metadata,omitempty"` // optional caption or extra info
}

type SearchRequest struct {
	UserID string
	Params struct {
		TargetGender string
		TargetAgeMin int
		TargetAgeMax int
	}
	ResultCh chan string // Channel для повернення RoomID
}
