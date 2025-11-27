package telegram

import (
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"log"
	"reflect"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Client struct {
	AnonID  string
	RoomID  string
	Hub     *chathub.ManagerService
	Send    chan models.ChatMessage
	BotAPI  *tgbotapi.BotAPI
	Storage storage.Storage
}

func (c *Client) GetAnonID() string                         { return c.AnonID }
func (c *Client) GetRoomID() string                         { return c.RoomID }
func (c *Client) SetRoomID(id string)                       { c.RoomID = id }
func (c *Client) GetSendChannel() chan<- models.ChatMessage { return c.Send }

func (c *Client) Run()   { go c.writePump() }
func (c *Client) Close() { close(c.Send) }

// --- Допоміжні функції ---

// setReplyID — скорочений варіант через reflection
func (c *Client) setReplyID(tgMsg tgbotapi.Chattable, originalHistoryID uint) tgbotapi.Chattable {
	if c.Storage == nil {
		return tgMsg
	}

	replyTgIDUint, err := c.Storage.FindPartnerTelegramIDForReply(originalHistoryID, c.AnonID)
	if err != nil || replyTgIDUint == nil {
		return tgMsg
	}
	replyTgID := int(*replyTgIDUint)

	v := reflect.ValueOf(tgMsg)

	// 🔹 Якщо це структура (value), створюємо адресне значення
	if v.Kind() == reflect.Struct {
		ptr := reflect.New(v.Type()) // *MessageConfig
		ptr.Elem().Set(v)            // копіюємо старі поля
		v = ptr                      // тепер v — pointer
	}

	if v.Kind() == reflect.Ptr {
		elem := v.Elem()
		field := elem.FieldByName("ReplyToMessageID")
		if field.IsValid() && field.CanSet() && field.Kind() == reflect.Int {
			field.SetInt(int64(replyTgID))
			return v.Interface().(tgbotapi.Chattable)
		}
	}

	return tgMsg
}

// escapeMarkdownV2 — залишено як заглушку
func escapeMarkdownV2(text string) string {
	return text
}

// --- Основна логіка ---
func (c *Client) writePump() {
	defer log.Printf("Зупинка writePump для Telegram клієнта %s", c.AnonID)

	for message := range c.Send {
		if message.SenderID == c.AnonID && message.Type != "system_info" {
			continue
		}

		chatID, _ := strconv.ParseInt(c.AnonID, 10, 64)
		if chatID == 0 {
			continue
		}

		tgMsg := c.buildTelegramMessage(chatID, message)
		if tgMsg == nil {
			continue
		}

		// Встановлення ReplyToMessageID
		if message.ReplyToMessageID != nil {
			tgMsg = c.setReplyID(tgMsg, *message.ReplyToMessageID)
		}

		// Відправка
		sentMsg, err := c.BotAPI.Send(tgMsg)
		if err != nil {
			log.Printf("ERROR: Failed to send Telegram message to %s: %v", c.AnonID, err)
			continue
		}

		// Збереження MessageID
		if message.ID != 0 && c.Storage != nil {
			if err := c.Storage.SaveTgMessageID(uint(message.ID), c.AnonID, sentMsg.MessageID); err != nil {
				log.Printf("ERROR: Failed to save Telegram Message ID %d for history %d: %v", sentMsg.MessageID, message.ID, err)
			}
		}
	}
}

func (c *Client) buildTelegramMessage(chatID int64, message models.ChatMessage) tgbotapi.Chattable {
	//const parseMode = tgbotapi.ModeMarkdownV2
	const parseMode = tgbotapi.ModeMarkdown
	content := escapeMarkdownV2(message.Content)
	//metadata := escapeMarkdownV2(message.Metadata)

	// --- 1. Обробка РЕДАГУВАННЯ (edit) ---
	if message.Type == "edit" {
		// Ми очікуємо, що Hub встановив TgMessageIDSender у TG ID повідомлення партнера, яке потрібно редагувати.
		if message.TgMessageIDSender == nil {
			log.Printf("ERROR: Cannot edit message without partner's TgMessageID. Sending as new message.")
			// Fallback: Відправити як нове повідомлення (стара логіка)
			msg := tgbotapi.NewMessage(chatID, "✏️ *Редаговано:*\n"+content)
			msg.ParseMode = parseMode
			return msg
		}

		tgIDToEdit := int(*message.TgMessageIDSender)

		// 1.1. Редагування Caption (якщо є Metadata, це медіа, Content - це новий Caption)
		if message.Metadata != "" {
			editConfig := tgbotapi.NewEditMessageCaption(
				chatID,
				tgIDToEdit,
				content, // Content - це НОВИЙ Caption
			)
			editConfig.ParseMode = parseMode
			return editConfig
		}

		// 1.2. Редагування тексту
		editConfig := tgbotapi.NewEditMessageText(
			chatID,
			tgIDToEdit,
			content,
		)
		editConfig.ParseMode = parseMode
		return editConfig
	}

	switch message.Type {
	case "text", "system_info":
		msg := tgbotapi.NewMessage(chatID, content)
		msg.ParseMode = parseMode
		return msg

	case "photo", "video", "animation":
		if message.ReplyToMessageID != nil {
			originalHistory, err := c.Storage.FindHistoryByID(*message.ReplyToMessageID)

			if err != nil || originalHistory == nil {
				log.Printf("ERROR: Failed to fetch original history record %d: %v", *message.ReplyToMessageID, err)
			}

			if originalHistory.Content == message.Content {
				msg := tgbotapi.NewMessage(chatID, message.Metadata)
				msg.ParseMode = parseMode
				return msg
			}

		}
		if message.Content == "" {
			log.Printf("ERROR: Media message (%s) missing FileID", message.Type)
			return nil
		}
		fileID := tgbotapi.FileID(message.Content)
		caption := escapeMarkdownV2(message.Metadata)

		switch message.Type {
		case "photo":
			msg := tgbotapi.NewPhoto(chatID, fileID)
			msg.Caption, msg.ParseMode = caption, parseMode
			return msg
		case "video":
			msg := tgbotapi.NewVideo(chatID, fileID)
			msg.Caption, msg.ParseMode = caption, parseMode
			return msg
		case "animation":
			msg := tgbotapi.NewAnimation(chatID, fileID)
			msg.Caption, msg.ParseMode = caption, parseMode
			return msg
		}

	case "sticker":
		return tgbotapi.NewSticker(chatID, tgbotapi.FileID(message.Content))

	case "voice":
		return tgbotapi.NewVoice(chatID, tgbotapi.FileID(message.Content))

	case "video_note":
		return tgbotapi.NewVideoNote(chatID, 0, tgbotapi.FileID(message.Content))

	case "system_search_start", "system_reconnect":
		msg := tgbotapi.NewMessage(chatID, content)
		msg.ParseMode = parseMode
		return msg

	case "system_match_found":
		c.RoomID = message.RoomID
		msg := tgbotapi.NewMessage(chatID, "✅ **Співрозмовника знайдено!** Починайте спілкування.")
		msg.ParseMode = parseMode
		return msg

	case "system_match_stop_self":
		c.RoomID = ""
		msg := tgbotapi.NewMessage(chatID, "🚪 **Чат завершено.** Ви вийшли з кімнати. Напишіть `/start`, щоб знайти нового співрозмовника.")
		msg.ParseMode = parseMode
		return msg

	case "system_match_stop_partner":
		c.RoomID = ""
		msg := tgbotapi.NewMessage(chatID, "🚫 **Чат завершено.** Співрозмовник покинув чат. Напишіть `/start`, щоб знайти нового співрозмовника.")
		msg.ParseMode = parseMode
		return msg

	default:
		log.Printf("Unhandled message type in buildTelegramMessage: %s", message.Type)
		msg := tgbotapi.NewMessage(chatID, "⚠️ Непідтримуваний тип повідомлення.")
		msg.ParseMode = parseMode
		return msg
	}

	return nil
}
