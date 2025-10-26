package telegram

import (
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotService struct {
	BotAPI *tgbotapi.BotAPI
	Hub    *chathub.ManagerService
}

func NewBotService(token string, hub *chathub.ManagerService) (*BotService, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	bot.Debug = false // Встановіть true для дебагу
	log.Printf("Authorized on account %s", bot.Self.UserName)
	return &BotService{BotAPI: bot, Hub: hub}, nil
}

// Run - це "ReadPump" для всіх Telegram-клієнтів
func (s *BotService) Run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := s.BotAPI.GetUpdatesChan(u)

	for update := range updates {
		// 1️⃣ Реакції (нове API Telegram)
		// todo: implement reactions when lib will allow

		// 2️⃣ Редагування повідомлень
		if update.EditedMessage != nil {
			msg := update.EditedMessage
			senderID := strconv.FormatInt(msg.From.ID, 10)

			chatMsg := models.ChatMessage{
				SenderID: senderID,
				RoomID:   strconv.FormatInt(msg.Chat.ID, 10),
				Type:     "edit",
				Content:  msg.Text,
			}

			// Якщо це було редагування відповіді на ботське повідомлення
			if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.IsBot {
				chatMsg.Type = "reply"
				chatMsg.Metadata = msg.ReplyToMessage.Text
			}

			s.Hub.IncomingCh <- chatMsg
			continue
		}

		// 3️⃣ Звичайні повідомлення
		if update.Message == nil {
			continue // Ігноруємо оновлення без повідомлень (редагування, статуси тощо)
		}

		msg := update.Message
		anonID := strconv.FormatInt(msg.Chat.ID, 10)

		// 🟢 1. Find or create a Telegram client
		c, ok := s.Hub.Clients[anonID]
		if !ok {
			c = &Client{
				AnonID: anonID,
				Hub:    s.Hub,
				Send:   make(chan models.ChatMessage, 10),
				BotAPI: s.BotAPI,
			}
			s.Hub.RegisterCh <- c
			go c.Run()
		}

		// 🟢 2. Create a ChatMessage
		chatMsg := models.ChatMessage{
			SenderID: anonID,
			RoomID:   c.GetRoomID(),
		}

		switch {
		case msg.Text != "":
			chatMsg.Type = "text"
			chatMsg.Content = msg.Text

			if msg.IsCommand() {
				switch msg.Command() {
				case "start":
					chatMsg.Type = "command_start"
				case "stop":
					chatMsg.Type = "command_stop"
				default:
					c.GetSendChannel() <- models.ChatMessage{
						Type:    "system_info",
						Content: "❌ Невідома команда. Використовуйте /start або /stop.",
					}
					continue
				}
			}

		case msg.Photo != nil:
			chatMsg.Type = "photo"
			largestPhoto := msg.Photo[len(msg.Photo)-1]
			chatMsg.Content = largestPhoto.FileID
			chatMsg.Metadata = msg.Caption

		case msg.Video != nil:
			chatMsg.Type = "video"
			chatMsg.Content = msg.Video.FileID
			chatMsg.Metadata = msg.Caption

		case msg.Sticker != nil:
			chatMsg.Type = "sticker"
			chatMsg.Content = msg.Sticker.FileID

		case msg.Voice != nil:
			chatMsg.Type = "voice"
			chatMsg.Content = msg.Voice.FileID

		case msg.Animation != nil:
			chatMsg.Type = "animation"
			chatMsg.Content = msg.Animation.FileID
			chatMsg.Metadata = msg.Caption

		case msg.VideoNote != nil:
			chatMsg.Type = "video_note"
			chatMsg.Content = msg.VideoNote.FileID

		default:
			c.GetSendChannel() <- models.ChatMessage{
				Type:    "system_info",
				Content: "⚠️ Цей тип повідомлення поки що не підтримується.",
			}
			continue
		}

		// 🟢 3. Reject messages if not in a room (and not a command)
		if chatMsg.RoomID == "" && !strings.HasPrefix(chatMsg.Type, "command_") {
			c.GetSendChannel() <- models.ChatMessage{
				Type:    "system_info",
				Content: "❌ Ви не перебуваєте в чаті. Напишіть /start, щоб знайти співрозмовника.",
			}
			continue
		}

		// 🟢 4. Forward message into Hub
		s.Hub.IncomingCh <- chatMsg
	}
}
