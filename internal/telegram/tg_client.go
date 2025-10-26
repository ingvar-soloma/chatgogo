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
		if message.SenderID == c.AnonID && message.Type != "system_info" {
			continue // не надсилаємо собі
		}

		// Конвертуємо AnonID (string) назад у ChatID (int64)
		chatID, _ := strconv.ParseInt(c.AnonID, 10, 64)
		if chatID == 0 {
			continue
		}

		var tgMsg tgbotapi.Chattable
		var parseMode = tgbotapi.ModeMarkdown

		switch message.Type {

		case "text":
			tgMsg = tgbotapi.NewMessage(chatID, message.Content)

		case "photo":
			// Пересилання фото за допомогою FileID (Content)
			photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(message.Content))
			photoMsg.Caption = message.Metadata // Додаємо підпис
			tgMsg = photoMsg

		case "sticker":
			// Пересилання стікера за допомогою FileID (Content)
			tgMsg = tgbotapi.NewSticker(chatID, tgbotapi.FileID(message.Content))

		case "video":
			// Пересилання відео за допомогою FileID (Content)
			videoMsg := tgbotapi.NewVideo(chatID, tgbotapi.FileID(message.Content))
			videoMsg.Caption = message.Metadata // Додаємо підпис
			tgMsg = videoMsg

		case "voice":
			// Пересилання голосового повідомлення за допомогою FileID (Content)
			tgMsg = tgbotapi.NewVoice(chatID, tgbotapi.FileID(message.Content))

		case "animation":
			animMsg := tgbotapi.NewAnimation(chatID, tgbotapi.FileID(message.Content))
			animMsg.Caption = message.Metadata
			tgMsg = animMsg

		case "video_note":
			tgMsg = tgbotapi.NewVideoNote(chatID, 0, tgbotapi.FileID(message.Content))

		case "edit":
			reply := tgbotapi.NewMessage(chatID, "✏️ *Редаговано:* "+message.Content)
			tgMsg = reply

		case "reply":
			reply := tgbotapi.NewMessage(chatID, "↩️ *Відповідь від співрозмовника:*\n"+message.Content)
			tgMsg = reply

		case "system_search_start":
			tgMsg = tgbotapi.NewMessage(chatID, message.Content)

		case "system_match_found":
			c.RoomID = message.RoomID
			tgMsg = tgbotapi.NewMessage(chatID, "✅ **Співрозмовника знайдено!** Починайте спілкування.")

		case "system_match_stop_self":
			c.RoomID = ""
			tgMsg = tgbotapi.NewMessage(chatID, "🚪 **Чат завершено.** Ви вийшли з кімнати. Напишіть `/start`, щоб знайти нового співрозмовника.")

		case "system_match_stop_partner":
			c.RoomID = ""
			tgMsg = tgbotapi.NewMessage(chatID, "🚫 **Чат завершено.** Співрозрозмовник покинув чат. Напишіть `/start`, щоб знайти нового співрозмовника.")

		case "system_info":
			tgMsg = tgbotapi.NewMessage(chatID, message.Content)

		default:
			// ⬅️ ОБРОБКА НЕПІДТРИМУВАНОГО ТИПУ ВІД HUB/МАТЧЕРА
			// Якщо системне повідомлення чи повідомлення від партнера має невідомий тип
			if message.SenderID != c.AnonID {
				log.Printf("Unhandled message type received from Hub for TG client %s: %s", c.AnonID, message.Type)
				// Надсилаємо попередження замість непідтримуваного типу
				tgMsg = tgbotapi.NewMessage(chatID, "⚠️ **Помилка пересилання.** Співрозмовник надіслав непідтримуваний або невідомий тип повідомлення.")
			} else {
				continue // Ігноруємо власні невідомі повідомлення
			}
		}

		// Відправка повідомлення
		if tgMsg != nil {
			// Встановлюємо ParseMode, якщо це Message (для Markdown)
			if msg, ok := tgMsg.(tgbotapi.MessageConfig); ok {
				msg.ParseMode = parseMode
				tgMsg = msg // Оновлюємо змінну
				msg.ReplyToMessageID = extractMessageID(message.Metadata)
			}

			if _, err := c.BotAPI.Send(tgMsg); err != nil {
				log.Printf("ERROR: Failed to send Telegram message of type %s to ChatID %d: %v", message.Type, chatID, err)
			}
		}
	}
}
