package chathub

import (
	"chatgogo/backend/internal/models"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// ReadPump читає повідомлення з WebSocket і передає їх у ChatHub.
func (c *Client) ReadPump() {
	// Встановлення таймаутів та обробка закриття з'єднання
	defer func() {
		c.Hub.UnregisterCh <- c // Надсилаємо команду на Unregister
		c.Conn.Close()          // Використовуємо експортоване поле Conn
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

		// Тут буде логіка декодування JSON та надсилання у Hub.IncomingCh
		var msg models.ChatMessage

		// !!! Додамо декодування JSON (критично важливий крок) !!!
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error decoding JSON from client %s: %v", c.AnonID, err)
			continue // Пропускаємо невірне повідомлення
		}

		msg.SenderID = c.AnonID

		// Надсилаємо повідомлення у головний канал хаба
		c.Hub.IncomingCh <- msg
	}
}

// WritePump читає повідомлення з каналу Send і записує їх у WebSocket.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		c.Conn.Close() // Використовуємо експортоване поле Conn
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

			// !!! Кодування JSON для відправки !!!
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

				// Кодування додаткових повідомлень
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
