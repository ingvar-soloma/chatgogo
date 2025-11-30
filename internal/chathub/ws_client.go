package chathub

import (
	"chatgogo/backend/internal/models"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

// WebSocketClient is an implementation of the Client interface for WebSocket connections.
type WebSocketClient struct {
	UserID string
	RoomID string
	Conn   *websocket.Conn
	Hub    *ManagerService
	Send   chan models.ChatMessage
}

// GetUserID returns the client's user ID.
func (c *WebSocketClient) GetUserID() string { return c.UserID }

// GetRoomID returns the ID of the room the client is in.
func (c *WebSocketClient) GetRoomID() string { return c.RoomID }

// SetRoomID sets the client's current room ID.
func (c *WebSocketClient) SetRoomID(id string) { c.RoomID = id }

// GetSendChannel returns the client's outbound message channel.
func (c *WebSocketClient) GetSendChannel() chan<- models.ChatMessage { return c.Send }

// Run starts the read and write pumps for the WebSocket client.
func (c *WebSocketClient) Run() {
	go c.writePump()
	go c.readPump()
}

// Close closes the client's send channel, which in turn gracefully
// shuts down the write pump and the WebSocket connection.
func (c *WebSocketClient) Close() {
	close(c.Send)
}

// readPump pumps messages from the WebSocket connection to the hub.
// It ensures that the client is unregistered and the connection is closed
// when the read loop exits.
func (c *WebSocketClient) readPump() {
	defer func() {
		c.Hub.UnregisterCh <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading message: %v", err)
			}
			break
		}

		var msg models.ChatMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error decoding JSON from client %s: %v", c.UserID, err)
			continue
		}
		msg.SenderID = c.UserID
		c.Hub.IncomingCh <- msg
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
// It also sends periodic ping messages to keep the connection alive.
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			dataToWrite, err := json.Marshal(message)
			if err != nil {
				log.Printf("Error encoding JSON for client %s: %v", c.UserID, err)
				continue
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(dataToWrite)

			// Batch write any pending messages in the channel for efficiency.
			n := len(c.Send)
			for i := 0; i < n; i++ {
				nextMsg := <-c.Send
				extraData, _ := json.Marshal(nextMsg)
				w.Write(extraData)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			// Send a ping message to keep the connection alive.
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
