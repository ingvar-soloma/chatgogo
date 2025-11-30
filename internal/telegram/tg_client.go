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

// Client implements the chathub.Client interface for Telegram users.
type Client struct {
	UserID  string // Internal UUID
	AnonID  string // Telegram Chat ID
	RoomID  string
	Hub     *chathub.ManagerService
	Send    chan models.ChatMessage
	BotAPI  *tgbotapi.BotAPI
	Storage storage.Storage
}

// GetUserID returns the client's internal user ID.
func (c *Client) GetUserID() string { return c.UserID }

// GetRoomID returns the ID of the room the client is in.
func (c *Client) GetRoomID() string { return c.RoomID }

// SetRoomID sets the client's current room ID.
func (c *Client) SetRoomID(id string) { c.RoomID = id }

// GetSendChannel returns the client's outbound message channel.
func (c *Client) GetSendChannel() chan<- models.ChatMessage { return c.Send }

// Run starts the client's write pump.
func (c *Client) Run() { go c.writePump() }

// Close closes the client's send channel.
func (c *Client) Close() { close(c.Send) }

// setReplyID sets the ReplyToMessageID field on a Telegram message if applicable.
func (c *Client) setReplyID(tgMsg tgbotapi.Chattable, originalHistoryID uint) tgbotapi.Chattable {
	if c.Storage == nil {
		return tgMsg
	}

	replyTgIDUint, err := c.Storage.FindPartnerTelegramIDForReply(originalHistoryID, c.UserID)
	if err != nil || replyTgIDUint == nil {
		return tgMsg
	}
	replyTgID := int(*replyTgIDUint)

	v := reflect.ValueOf(tgMsg)
	if v.Kind() == reflect.Struct {
		ptr := reflect.New(v.Type())
		ptr.Elem().Set(v)
		v = ptr
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

// escapeMarkdownV2 is a placeholder for a function that would escape text for Telegram's MarkdownV2 parse mode.
func escapeMarkdownV2(text string) string {
	return text
}

// writePump pumps messages from the hub to the Telegram user.
func (c *Client) writePump() {
	defer log.Printf("Stopping writePump for Telegram client %s (User: %s)", c.AnonID, c.UserID)

	for message := range c.Send {
		if message.SenderID == c.UserID && message.Type != "system_info" {
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

		if message.ReplyToMessageID != nil {
			tgMsg = c.setReplyID(tgMsg, *message.ReplyToMessageID)
		}

		sentMsg, err := c.BotAPI.Send(tgMsg)
		if err != nil {
			log.Printf("ERROR: Failed to send Telegram message to %s: %v", c.AnonID, err)
			continue
		}

		if message.ID != 0 && c.Storage != nil {
			if err := c.Storage.SaveTgMessageID(uint(message.ID), c.UserID, sentMsg.MessageID); err != nil {
				log.Printf("ERROR: Failed to save Telegram Message ID %d for history %d: %v", sentMsg.MessageID, message.ID, err)
			}
		}
	}
}

// buildTelegramMessage constructs a `tgbotapi.Chattable` from a `models.ChatMessage`.
func (c *Client) buildTelegramMessage(chatID int64, message models.ChatMessage) tgbotapi.Chattable {
	const parseMode = tgbotapi.ModeMarkdown
	content := escapeMarkdownV2(message.Content)

	if message.Type == "edit" {
		if message.TgMessageIDSender == nil {
			log.Printf("ERROR: Cannot edit message without partner's TgMessageID. Sending as new message.")
			msg := tgbotapi.NewMessage(chatID, "✏️ *Edited:*\n"+content)
			msg.ParseMode = parseMode
			return msg
		}
		tgIDToEdit := int(*message.TgMessageIDSender)

		if message.Metadata != "" {
			editConfig := tgbotapi.NewEditMessageCaption(chatID, tgIDToEdit, content)
			editConfig.ParseMode = parseMode
			return editConfig
		}
		editConfig := tgbotapi.NewEditMessageText(chatID, tgIDToEdit, content)
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
		msg := tgbotapi.NewMessage(chatID, "✅ **Match found!** Start chatting.")
		msg.ParseMode = parseMode
		return msg
	case "system_match_stop_self":
		c.RoomID = ""
		msg := tgbotapi.NewMessage(chatID, "🚪 **Chat ended.** You left the room. Type /start to find a new partner.")
		msg.ParseMode = parseMode
		return msg
	case "system_match_stop_partner":
		c.RoomID = ""
		msg := tgbotapi.NewMessage(chatID, "🚫 **Chat ended.** Your partner left the chat. Type /start to find a new partner.")
		msg.ParseMode = parseMode
		return msg
	default:
		log.Printf("Unhandled message type in buildTelegramMessage: %s", message.Type)
		msg := tgbotapi.NewMessage(chatID, "⚠️ Unsupported message type.")
		msg.ParseMode = parseMode
		return msg
	}
	return nil
}
