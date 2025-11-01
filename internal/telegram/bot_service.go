package telegram

import (
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotService struct {
	BotAPI  *tgbotapi.BotAPI
	Hub     *chathub.ManagerService
	Storage storage.Storage
}

func NewBotService(token string, hub *chathub.ManagerService, s storage.Storage) (*BotService, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	bot.Debug = false // Встановіть true для дебагу
	log.Printf("Authorized on account %s", bot.Self.UserName)
	return &BotService{BotAPI: bot, Hub: hub, Storage: s}, nil
}

// Run - це "ReadPump" для всіх Telegram-клієнтів
func (s *BotService) Run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := s.BotAPI.GetUpdatesChan(u)

	for update := range updates {
		// 1️⃣ Реакції (нове API Telegram)
		// todo: implement reactions when api and lib will allow

		// 2️⃣ Редагування повідомлень
		if update.EditedMessage != nil {
			msg := update.EditedMessage
			anonID := strconv.FormatInt(msg.Chat.ID, 10)

			// Гарантуємо наявність клієнта, щоб отримати поточну кімнату
			c, ok := s.Hub.Clients[anonID]
			if !ok {
				c = &Client{
					AnonID:  anonID,
					Hub:     s.Hub,
					Send:    make(chan models.ChatMessage, 10),
					BotAPI:  s.BotAPI,
					Storage: s.Storage,
				}
				s.Hub.RegisterCh <- c
				go c.Run()
			}

			var tgMessageIDSender *uint
			tempID := uint(msg.MessageID)

			// 4. Беремо адресу тимчасової змінної, щоб отримати *uint
			tgMessageIDSender = &tempID

			chatMsg := models.ChatMessage{
				SenderID:          anonID,
				TgMessageIDSender: tgMessageIDSender,
				RoomID:            c.GetRoomID(),
				Type:              "edit",
				Content:           msg.Text,
			}

			// 1. Отримуємо Telegram Message ID, яке відредаговане
			editedTGID := uint(msg.MessageID)

			// 2. ЗНАЙТИ ВНУТРІШНІЙ CHAT HISTORY ID ЗА TG ID
			originalHistoryID, err := s.Storage.FindOriginalHistoryIDByTgID(editedTGID)

			if err != nil {
				log.Printf("ERROR: Failed to find original history ID: %v", err)
				// Можемо продовжити без реплаю
			} else if originalHistoryID != nil {
				// Встановлюємо ChatHistory.ID як посилання на реплай
				chatMsg.ReplyToMessageID = originalHistoryID
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
				AnonID:  anonID,
				Hub:     s.Hub,
				Send:    make(chan models.ChatMessage, 10),
				BotAPI:  s.BotAPI,
				Storage: s.Storage,
			}
			s.Hub.RegisterCh <- c
			go c.Run()
		}

		// 🟢 2. Create a ChatMessage
		// 1. Оголошуємо змінну типу *uint (вона за замовчуванням буде nil)
		var tgMessageIDSender *uint
		// 2. Перевіряємо, чи є MessageID валідним (> 0)
		if msg.MessageID > 0 {
			// 3. Конвертуємо int у uint і зберігаємо в тимчасовій змінній
			tempID := uint(msg.MessageID)

			// 4. Беремо адресу тимчасової змінної, щоб отримати *uint
			tgMessageIDSender = &tempID
		}

		chatMsg := models.ChatMessage{
			TgMessageIDSender: tgMessageIDSender,
			SenderID:          anonID,
			RoomID:            c.GetRoomID(),
		}

		// Якщо користувач відповів на повідомлення
		if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil {
			// 1. Отримуємо Telegram Message ID, на яке відповіли
			replyTGID := uint(msg.ReplyToMessage.MessageID)

			// 2. ЗНАЙТИ ВНУТРІШНІЙ CHAT HISTORY ID ЗА TG ID
			originalHistoryID, err := s.Storage.FindOriginalHistoryIDByTgID(replyTGID)

			if err != nil {
				log.Printf("ERROR: Failed to find original history ID: %v", err)
				// Можемо продовжити без реплаю
			} else if originalHistoryID != nil {
				// Встановлюємо ChatHistory.ID як посилання на реплай
				chatMsg.ReplyToMessageID = originalHistoryID
			}
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

				case "next":
					chatMsg.Type = "command_next"

				case "settings":
					chatMsg.Type = "command_settings"

				case "report":
					chatMsg.Type = "command_report"

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
				Content: "⚠️ Цей тип повідомлення не підтримується.",
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
