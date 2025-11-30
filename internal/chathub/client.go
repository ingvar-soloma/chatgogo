package chathub

import "chatgogo/backend/internal/models"

// Client is the interface for any type of connection (e.g., WebSocket, Telegram).
// It abstracts the underlying communication mechanism, allowing the hub to manage
// different client types uniformly.
type Client interface {
	// GetUserID returns the unique identifier for the user associated with the client.
	GetUserID() string
	// GetRoomID returns the identifier of the chat room the client is currently in.
	GetRoomID() string
	// SetRoomID assigns the client to a specific chat room. This is typically called
	// by the MatcherService after a successful match.
	SetRoomID(string)

	// GetSendChannel returns the channel to which the ManagerService (hub) sends
	// messages intended for this specific client. It is a send-only channel.
	GetSendChannel() chan<- models.ChatMessage

	// Run starts the client's read and write pumps, which handle incoming and
	// outgoing messages.
	Run()
	// Close gracefully shuts down the client's connection and associated channels.
	Close()
}
