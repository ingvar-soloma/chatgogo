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

// BotService відповідає за прийом оновлень Telegram і маршрутизацію у хаб
type BotService struct {
	BotAPI  *tgbotapi.BotAPI
	Hub     *chathub.ManagerService
	Storage storage.Storage
}

// NewBotService створює новий Telegram Bot Service
func NewBotService(token string, hub *chathub.ManagerService, s storage.Storage) (*BotService, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	bot.Debug = false
	log.Printf("✅ Authorized on account %s", bot.Self.UserName)

	return &BotService{BotAPI: bot, Hub: hub, Storage: s}, nil
}

// --- Допоміжні функції ---

// extractMessageContent уніфіковано витягує текст або caption
func extractMessageContent(msg *tgbotapi.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Text != "" {
		return msg.Text
	}
	if msg.Caption != "" {
		return msg.Caption
	}
	return ""
}

// getOrCreateClient повертає існуючого або створює нового Telegram-клієнта
func (s *BotService) getOrCreateClient(chatID int64) *Client {
	anonID := strconv.FormatInt(chatID, 10)

	// 1. Перевіряємо, чи клієнт вже існує в хабі
	if existingClient, ok := s.Hub.Clients[anonID]; ok {

		// 2. Виконуємо БЕЗПЕЧНЕ затвердження типу
		if client, ok := existingClient.(*Client); ok {
			return client
		}
		// Це не повинно статися, але це захист
		log.Printf("ERROR: Client %s is not of type *telegram.Client", anonID)
	}

	// 3. Клієнт не існує, створюємо нового
	newClient := &Client{
		AnonID:  anonID,
		Hub:     s.Hub,
		Send:    make(chan models.ChatMessage, 10),
		BotAPI:  s.BotAPI,
		Storage: s.Storage,
	}

	// 4. Реєструємо клієнта в хабі (хаб зберігає його як chathub.Client)
	s.Hub.RegisterCh <- newClient

	// 5. Запускаємо goroutine (метод Run() належить типу *Client)
	go newClient.Run()

	// 6. Повертаємо конкретний тип *Client (без затвердження)
	return newClient
}

// handleEditedMessage обробляє відредаговані повідомлення
func (s *BotService) handleEditedMessage(msg *tgbotapi.Message) {
	anonID := strconv.FormatInt(msg.Chat.ID, 10)
	c := s.getOrCreateClient(msg.Chat.ID)

	content := extractMessageContent(msg)
	if content == "" {
		log.Println("Ignoring media edit without caption.")
		return // Ігноруємо редагування медіа без зміни тексту
	}

	tempID := uint(msg.MessageID)

	chatMsg := models.ChatMessage{
		SenderID:          anonID,
		TgMessageIDSender: &tempID,
		RoomID:            c.GetRoomID(),
		Type:              "edit",
		Content:           content,
	}

	editedTGID := uint(msg.MessageID)
	originalHistoryID, err := s.Storage.FindOriginalHistoryIDByTgID(editedTGID)
	if err != nil {
		log.Printf("ERROR: FindOriginalHistoryIDByTgID failed: %v", err)
	} else if originalHistoryID != nil {
		chatMsg.ReplyToMessageID = originalHistoryID
	}

	s.Hub.IncomingCh <- chatMsg
}

// handleIncomingMessage обробляє нові повідомлення користувачів
func (s *BotService) handleIncomingMessage(msg *tgbotapi.Message) {
	anonID := strconv.FormatInt(msg.Chat.ID, 10)
	c := s.getOrCreateClient(msg.Chat.ID)

	tempID := uint(msg.MessageID)
	chatMsg := models.ChatMessage{
		TgMessageIDSender: &tempID,
		SenderID:          anonID,
		RoomID:            c.GetRoomID(),
	}

	// Обробка відповіді (reply)
	if msg.ReplyToMessage != nil {
		replyTGID := uint(msg.ReplyToMessage.MessageID)
		if originalHistoryID, err := s.Storage.FindOriginalHistoryIDByTgID(replyTGID); err == nil && originalHistoryID != nil {
			chatMsg.ReplyToMessageID = originalHistoryID
		}
	}

	switch {
	case msg.Text != "":
		chatMsg.Content = msg.Text
		s.handleTextMessage(c, msg, &chatMsg) // Передаємо chatMsg для модифікації

	case msg.Photo != nil:
		largestPhoto := msg.Photo[len(msg.Photo)-1]
		chatMsg.Type = "photo"
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
		return
	}

	// Перевірка, чи користувач у чаті
	if chatMsg.RoomID == "" && !strings.HasPrefix(chatMsg.Type, "command_") {
		c.GetSendChannel() <- models.ChatMessage{
			Type:    "system_info",
			Content: "❌ Ви не перебуваєте в чаті. Напишіть /start, щоб знайти співрозмовника.",
		}
		return
	}

	s.Hub.IncomingCh <- chatMsg
}

// handleTextMessage обробляє текстові повідомлення та команди
func (s *BotService) handleTextMessage(c *Client, msg *tgbotapi.Message, chatMsg *models.ChatMessage) {
	chatMsg.Type = "text" // За замовчуванням

	if !msg.IsCommand() {
		return // Це звичайне текстове повідомлення, тип вже "text"
	}

	// Якщо це команда, оновлюємо тип
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
		// Якщо команда невідома, відправляємо відповідь і не передаємо у хаб
		// Для цього треба змінити логіку в handleIncomingMessage,
		// але поки що просто встановимо тип "unknown_command"
		chatMsg.Type = "unknown_command"
		c.GetSendChannel() <- models.ChatMessage{
			Type:    "system_info",
			Content: "❌ Невідома команда. Використовуйте /start або /stop.",
		}
	}
}

// Run — головний цикл отримання Telegram-оновлень
func (s *BotService) Run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := s.BotAPI.GetUpdatesChan(u)

	for update := range updates {
		switch {
		case update.EditedMessage != nil:
			s.handleEditedMessage(update.EditedMessage)

		case update.Message != nil:
			s.handleIncomingMessage(update.Message)

			// default: ігноруємо інші типи оновлень
		}
	}
}
