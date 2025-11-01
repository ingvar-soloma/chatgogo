package telegram

import (
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"log"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Client реалізує інтерфейс chathub.Client
type Client struct {
	AnonID  string // Це буде ChatID юзера (як string)
	RoomID  string
	Hub     *chathub.ManagerService
	Send    chan models.ChatMessage
	BotAPI  *tgbotapi.BotAPI
	Storage storage.Storage
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
		//var parseMode = tgbotapi.ModeMarkdownV2
		var parseMode = tgbotapi.ModeMarkdown

		// --- 1. Створення об'єкта Telegram-повідомлення ---
		switch message.Type {

		case "text", "system_info":
			msg := tgbotapi.NewMessage(chatID, message.Content)
			msg.ParseMode = parseMode
			tgMsg = msg

		case "edit":
			msg := tgbotapi.NewMessage(chatID, "✏️ *Редаговано:*\n"+message.Content)
			msg.ParseMode = parseMode
			tgMsg = msg

		// case "reply" видалено, оскільки це тепер властивість, а не тип.

		case "photo":
			// Пересилання фото за допомогою FileID (Content)
			photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(message.Content))
			photoMsg.Caption = message.Metadata // Додаємо підпис
			photoMsg.ParseMode = parseMode      // Встановлюємо ParseMode для підпису
			tgMsg = photoMsg

		case "sticker":
			// StickerConfig підтримує ReplyToMessageID
			tgMsg = tgbotapi.NewSticker(chatID, tgbotapi.FileID(message.Content))

		case "video":
			// Пересилання відео за допомогою FileID (Content)
			videoMsg := tgbotapi.NewVideo(chatID, tgbotapi.FileID(message.Content))
			videoMsg.Caption = message.Metadata
			videoMsg.ParseMode = parseMode
			tgMsg = videoMsg

		case "voice":
			// VoiceConfig підтримує ReplyToMessageID
			tgMsg = tgbotapi.NewVoice(chatID, tgbotapi.FileID(message.Content))

		case "animation":
			animMsg := tgbotapi.NewAnimation(chatID, tgbotapi.FileID(message.Content))
			animMsg.Caption = message.Metadata
			animMsg.ParseMode = parseMode
			tgMsg = animMsg

		case "video_note":
			// VideoNoteConfig підтримує ReplyToMessageID
			tgMsg = tgbotapi.NewVideoNote(chatID, 0, tgbotapi.FileID(message.Content))

		case "system_search_start":
			msg := tgbotapi.NewMessage(chatID, message.Content)
			msg.ParseMode = parseMode
			tgMsg = msg

		case "system_match_found":
			c.RoomID = message.RoomID
			text := "✅ **Співрозмовника знайдено!** Починайте спілкування."
			msg := tgbotapi.NewMessage(chatID, escapeMarkdownV2(text))
			msg.ParseMode = parseMode
			tgMsg = msg

		case "system_match_stop_self":
			c.RoomID = ""
			text := "🚪 **Чат завершено.** Ви вийшли з кімнати. Напишіть `/start`, щоб знайти нового співрозмовника."
			msg := tgbotapi.NewMessage(chatID, escapeMarkdownV2(text))
			msg.ParseMode = parseMode
			tgMsg = msg

		case "system_match_stop_partner":
			c.RoomID = ""
			text := "🚫 **Чат завершено.** Співрозрозмовник покинув чат. Напишіть `/start`, щоб знайти нового співрозмовника."
			msg := tgbotapi.NewMessage(chatID, escapeMarkdownV2(text))
			msg.ParseMode = parseMode
			tgMsg = msg

		default:
			// ⬅️ ОБРОБКА НЕПІДТРИМУВАНОГО ТИПУ ВІД HUB/МАТЧЕРА
			// Якщо системне повідомлення чи повідомлення від партнера має невідомий тип
			if message.SenderID != c.AnonID {
				log.Printf("Unhandled message type received from Hub for TG client %s: %s", c.AnonID, message.Type)
				// Надсилаємо попередження замість непідтримуваного типу
				text := "⚠️ **Помилка пересилання.** Співрозмовник надіслав непідтримуваний або невідомий тип повідомлення."
				msg := tgbotapi.NewMessage(chatID, escapeMarkdownV2(text))
				msg.ParseMode = parseMode
				tgMsg = msg
			} else {
				continue // Ігноруємо власні невідомі повідомлення
			}
		}

		// --- 2. ВСТАНОВЛЕННЯ ВЛАСТИВОСТІ REPLY_TO_MESSAGE_ID (Загальна логіка) ---
		if tgMsg != nil && message.ReplyToMessageID != nil {
			originalHistoryID := *message.ReplyToMessageID // ВНУТРІШНІЙ ID

			if c.Storage == nil {
				log.Printf("WARN: Storage is nil in Telegram client %s, cannot resolve ReplyToMessageID for history %d", c.AnonID, originalHistoryID)
			} else {
				// 1. Знаходимо Telegram ID партнера
				replyTgID_uint, err := c.Storage.FindPartnerTelegramIDForReply(originalHistoryID, c.AnonID)
				if err != nil {
					log.Printf("ERROR: Failed to find partner TG Reply ID for history ID %d: %v", originalHistoryID, err)
				} else if replyTgID_uint != nil {
					// Конвертуємо *uint у int для API Telegram
					replyTgID := int(*replyTgID_uint)

					// 2. Встановлюємо ReplyToMessageID для об'єкта Telegram API.
					// Використовуємо кастинг для доступу до поля у відповідних конфігураціях.
					// Важливо: Присвоюємо змінену структуру назад до tgMsg.

					// MessageConfig (text, system_info, edit)
					if msg, ok := tgMsg.(tgbotapi.MessageConfig); ok {
						msg.ReplyToMessageID = replyTgID
						tgMsg = msg
						// PhotoConfig
					} else if msg, ok := tgMsg.(tgbotapi.PhotoConfig); ok {
						msg.ReplyToMessageID = replyTgID
						tgMsg = msg
						// VideoConfig
					} else if msg, ok := tgMsg.(tgbotapi.VideoConfig); ok {
						msg.ReplyToMessageID = replyTgID
						tgMsg = msg
						// StickerConfig
					} else if msg, ok := tgMsg.(tgbotapi.StickerConfig); ok {
						msg.ReplyToMessageID = replyTgID
						tgMsg = msg
						// VoiceConfig
					} else if msg, ok := tgMsg.(tgbotapi.VoiceConfig); ok {
						msg.ReplyToMessageID = replyTgID
						tgMsg = msg
						// AnimationConfig
					} else if msg, ok := tgMsg.(tgbotapi.AnimationConfig); ok {
						msg.ReplyToMessageID = replyTgID
						tgMsg = msg
						// VideoNoteConfig
					} else if msg, ok := tgMsg.(tgbotapi.VideoNoteConfig); ok {
						msg.ReplyToMessageID = replyTgID
						tgMsg = msg
					}
				}
			}
		}

		// --- 3. ВІДПРАВКА ---
		if tgMsg != nil {
			sentMsg, err := c.BotAPI.Send(tgMsg)
			if err != nil {
				log.Printf("ERROR: Failed to send Telegram message...: %v", err)
				continue
			}

			// 4. ЗБЕРЕЖЕННЯ ВЛАСНОГО TG Message ID У CHAT HISTORY
			if message.ID != 0 {
				// c.AnonID - це ID одержувача (бо ми відфільтрували відправника)
				if c.Storage == nil {
					log.Printf("WARN: Storage is nil, cannot SaveTgMessageID for history %d (AnonID %s, TG %d)", message.ID, c.AnonID, sentMsg.MessageID)
				} else {
					if err := c.Storage.SaveTgMessageID(uint(message.ID), c.AnonID, sentMsg.MessageID); err != nil {
						log.Printf("ERROR: Failed to save Telegram Message ID %d for history %d: %v", sentMsg.MessageID, message.ID, err)
					}
				}
			}
		}
	}
}

// escapeMarkdownV2 екранує всі зарезервовані символи MarkdownV2,
// окрім тих, що використовуються для форматування (*, _, `, [),
// щоб уникнути пошкодження вже існуючого форматування.
func escapeMarkdownV2(text string) string {
	//replacer := strings.NewReplacer(
	//	"\\", "\\\\",
	//	"|", "\\|",
	//	"{", "\\{",
	//	"}", "\\}",
	//	"(", "\\(",
	//	")", "\\)",
	//	">", "\\>",
	//	"#", "\\#",
	//	"+", "\\+",
	//	"-", "\\-",
	//	"=", "\\=",
	//	".", "\\.", // Екрануємо крапку
	//	"!", "\\!", // Екрануємо знак оклику
	//)
	// НЕ екрануємо *, _ або [
	//return replacer.Replace(text)
	return text
}
