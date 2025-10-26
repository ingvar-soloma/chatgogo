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

		case "system_search_start":
			content = message.Content

		case "system_match_found":
			// !! Важливо: Matcher має надіслати це повідомлення
			// І ми маємо оновити RoomID тут
			c.RoomID = message.RoomID
			content = "✅ Співрозмовника знайдено! Починайте спілкування."

		case "system_match_stop_self":
			c.RoomID = "" // Виходимо з кімнати
			content = "🚪 **Чат завершено.** Ви вийшли з кімнати. Напишіть `/start`, щоб знайти нового співрозмовника."

		case "system_match_stop_partner":
			c.RoomID = "" // Виходимо з кімнати
			content = "🚫 **Чат завершено.** Співрозмовник покинув чат. Напишіть `/start`, щоб знайти нового співрозмовника."

		case "system_info":
			// Для повідомлень типу "Ви не в чаті"
			content = message.Content

		// Додайте інші системні повідомлення (ban, search_start тощо)

		default:
			if message.SenderID != c.AnonID && message.SenderID != "system" {
				content = "ℹ️ Співрозмовник надіслав повідомлення, яке не підтримується у Telegram (наприклад, стікер або фото)."
			} else {
				// Це системне повідомлення, яке ми не знаємо, як обробити
				log.Printf("Unhandled system message type for TG client %s: %s", c.AnonID, message.Type)
				continue // Не турбувати користувача
			}
		}

		if content != "" {
			msg := tgbotapi.NewMessage(chatID, content)
			msg.ParseMode = tgbotapi.ModeMarkdown

			if _, err := c.BotAPI.Send(msg); err != nil {
				log.Printf("ERROR: Failed to send message to Telegram ChatID %d: %v", chatID, err)
			}
		}
	}
}
