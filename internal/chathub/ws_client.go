package chathub

import (
	"chatgogo/backend/internal/models"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// Константи, які були в manager.go або client_pump.go
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// WebSocketClient реалізує інтерфейс chathub.Client
type WebSocketClient struct {
	AnonID string
	RoomID string
	Conn   *websocket.Conn
	Hub    *ManagerService
	Send   chan models.ChatMessage
}

// --- Реалізація методів інтерфейсу ---

func (c *WebSocketClient) GetAnonID() string                         { return c.AnonID }
func (c *WebSocketClient) GetRoomID() string                         { return c.RoomID }
func (c *WebSocketClient) SetRoomID(id string)                       { c.RoomID = id }
func (c *WebSocketClient) GetSendChannel() chan<- models.ChatMessage { return c.Send }

// Run запускає 'pumps' для WebSocket
func (c *WebSocketClient) Run() {
	go c.writePump()
	go c.readPump()
}

// Close закриває Send канал (що зупинить writePump)
func (c *WebSocketClient) Close() {
	close(c.Send)
	// readPump зупиниться сам, коли Conn.Close() буде викликано в його defer
}

// --- Логіка 'Pump' (перейменовані ReadPump/WritePump) ---

func (c *WebSocketClient) readPump() {
	// Встановлення таймаутів та обробка закриття з'єднання
	defer func() {
		c.Hub.UnregisterCh <- c // Надсилаємо команду на Unregister
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		// Використовуємо метод ReadMessage від gorilla/websocket
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading message: %v", err)
			}
			break
		}

		var msg models.ChatMessage

		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error decoding JSON from client %s: %v", c.AnonID, err)
			continue // Пропускаємо невірне повідомлення
		}

		msg.SenderID = c.AnonID

		// Надсилаємо повідомлення у головний канал хаба
		c.Hub.IncomingCh <- msg
	}
}

// writePump (маленька 'w') читає повідомлення з каналу Send і записує їх у WebSocket.
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
				// Канал закрито хабом, закриваємо з'єднання WS
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			dataToWrite, err := json.Marshal(message)
			if err != nil {
				log.Printf("Error encoding JSON for client %s: %v", c.AnonID, err)
				continue
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(dataToWrite)

			// Перевіряємо, чи є ще повідомлення у каналі (для ефективності)
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
			// Надсилаємо Ping для підтримки з'єднання активним
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
