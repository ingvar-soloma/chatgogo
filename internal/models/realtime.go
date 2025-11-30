package models

// ChatMessage is the real-time, in-memory representation of a message.
// It is used for communication between different parts of the application,
// such as routing through the central hub and publishing to Redis.
type ChatMessage struct {
	// ID is the unique identifier of the message, populated after it's saved.
	ID uint `json:"id,omitempty"`
	// ReplyToMessageID points to the original message's ID in a reply chain.
	ReplyToMessageID *uint `json:"reply_to_message_id,omitempty"`
	// TgMessageIDSender is the Telegram-specific message ID for the sender.
	TgMessageIDSender *uint `json:"tg_message_id_sender,omitempty"`
	// SenderID is the anonymous ID of the user sending the message.
	SenderID string `json:"sender_id"`
	// RoomID is the identifier of the chat room.
	RoomID string `json:"room_id"`
	// Content holds the main body of the message.
	Content string `json:"content"`
	// Type specifies the kind of message (e.g., "text", "photo").
	Type string `json:"type"`
	// Metadata contains optional extra information, like a caption.
	Metadata string `json:"metadata,omitempty"`
}

// SearchRequest represents a user's request to find a chat partner.
// It is used by the matchmaking service to queue and pair users.
type SearchRequest struct {
	// UserID is the anonymous ID of the user initiating the search.
	UserID string
	// Params contains the search criteria for a chat partner.
	Params struct {
		TargetGender string
		TargetAgeMin int
		TargetAgeMax int
	}
	// ResultCh is a channel used to send the RoomID back to the user's session
	// once a match is found.
	ResultCh chan string
}
