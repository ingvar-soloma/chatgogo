package telegram

import (
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
	"log"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Client реалізує інтерфейс chathub.Client
type Client struct {
	AnonID string // Це буде ChatID юзера (як string)
	RoomID string
	Hub    *chathub.ManagerService
	Send   chan models.ChatMessage
	BotAPI *tgbotapi.BotAPI
}

// --- Реалізація методів інтерфейсу ---

func (c *Client) GetAnonID() string                         { return c.AnonID }
func (c *Client) GetRoomID() string                         { return c.RoomID }
func (c *Client) SetRoomID(id string)                       { c.RoomID = id }
func (c *Client) GetSendChannel() chan<- models.ChatMessage { return c.Send }

// Run запускає 'write pump'. 'Read pump' обробляється централізовано.
func (c *Client) Run() {
	go c.writePump()
}

// Close закриває Send канал
func (c *Client) Close() {
	close(c.Send)
}

// writePump слухає канал Send і надсилає повідомлення в Telegram
func (c *Client) writePump() {
	defer func() {
		log.Printf("Зупинка writePump для Telegram клієнта %s", c.AnonID)
	}()

	for message := range c.Send {
		// Конвертуємо AnonID (string) назад у ChatID (int64)
		chatID, _ := strconv.ParseInt(c.AnonID, 10, 64)
		if chatID == 0 {
			continue
		}

		var content string

		// Обробляємо різні типи повідомлень
		switch message.Type {
		case "text":
			// Не надсилаємо власні повідомлення назад собі
			if message.SenderID == c.AnonID {
				continue
			}
			content = message.Content

		case "system_match_found":
			// !! Важливо: Matcher має надіслати це повідомлення
			// І ми маємо оновити RoomID тут
			c.RoomID = message.RoomID
			content = "✅ Співрозмовника знайдено! Починайте спілкування."

		case "system_match_left":
			c.RoomID = "" // Виходимо з кімнати
			content = "🚫 Співрозмовник покинув чат."

		// Додайте інші системні повідомлення (ban, search_start тощо)

		default:
			continue // Не надсилаємо невідомі типи
		}

		if content != "" {
			msg := tgbotapi.NewMessage(chatID, content)
			c.BotAPI.Send(msg)
		}
	}
}
