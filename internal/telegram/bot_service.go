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

	// 1. Resolve UserID from DB (TelegramID -> UserID)
	user, err := s.Storage.SaveUserIfNotExists(anonID)
	if err != nil {
		log.Printf("FATAL: Failed to get/create user for TelegramID %s: %v", anonID, err)
		return nil
	}
	userID := user.ID

	// 2. Перевіряємо, чи клієнт вже існує в хабі (by UserID)
	if existingClient, ok := s.Hub.Clients[userID]; ok {

		// 3. Виконуємо БЕЗПЕЧНЕ затвердження типу
		if client, ok := existingClient.(*Client); ok {
			return client
		}
		// Це не повинно статися, але це захист
		log.Printf("ERROR: Client %s (User: %s) is not of type *telegram.Client", anonID, userID)
	}

	// 4. Клієнт не існує, створюємо нового
	newClient := &Client{
		UserID:  userID,
		AnonID:  anonID,
		Hub:     s.Hub,
		Send:    make(chan models.ChatMessage, 10),
		BotAPI:  s.BotAPI,
		Storage: s.Storage,
	}

	// --- FIX: Synchronously restore active room to avoid race condition ---
	// Note: GetActiveRoomIDForUser now expects UserID (UUID)
	activeRoomID, err := s.Storage.GetActiveRoomIDForUser(userID)
	if err == nil && activeRoomID != "" {
		newClient.SetRoomID(activeRoomID)
		log.Printf("Client %s (User: %s) restored to room %s synchronously.", anonID, userID, activeRoomID)
	}

	// 5. Реєструємо клієнта в хабі (хаб зберігає його як chathub.Client, key = UserID)
	// We need to ensure Hub registers with UserID key.
	// Hub.RegisterCh receives Client interface. ManagerService uses client.GetUserID() as key.
	// So we just send the client.
	s.Hub.RegisterCh <- newClient

	// 6. Запускаємо goroutine (метод Run() належить типу *Client)
	go newClient.Run()

	// 7. Повертаємо конкретний тип *Client (без затвердження)
	return newClient
}

// RestoreActiveSessions відновлює сесії для користувачів, які мають активні кімнати
func (s *BotService) RestoreActiveSessions() {
	log.Println("Restoring active Telegram sessions...")
	roomIDs, err := s.Storage.GetActiveRoomIDs()
	if err != nil {
		log.Printf("Failed to get active rooms: %v", err)
		return
	}

	for _, roomID := range roomIDs {
		room, err := s.Storage.GetRoomByID(roomID)
		if err != nil {
			continue
		}

		// Helper to restore user
		restoreUser := func(userIDStr string) {
			chatID, err := strconv.ParseInt(userIDStr, 10, 64)
			if err != nil {
				log.Printf("Invalid Telegram ID %s: %v", userIDStr, err)
				return
			}
			// Це створить клієнта і зареєструє його в Hub
			s.getOrCreateClient(chatID)
		}

		restoreUser(room.User1ID)
		restoreUser(room.User2ID)
	}
	log.Println("Active Telegram sessions restored.")
}

// handleEditedMessage обробляє відредаговані повідомлення (ОНОВЛЕНО)
func (s *BotService) handleEditedMessage(msg *tgbotapi.Message) {
	// anonID := strconv.FormatInt(msg.Chat.ID, 10) // Unused now
	c := s.getOrCreateClient(msg.Chat.ID)

	// 1. Знаходимо оригінальний ID історії
	editedTGID := uint(msg.MessageID)
	originalHistoryID, err := s.Storage.FindOriginalHistoryIDByTgIDMedia(editedTGID)
	if err != nil || originalHistoryID == nil {
		log.Printf("ERROR/WARN: Ignoring edit for un-tracked TG ID %d: %v", editedTGID, err)
		return
	}

	// 2. Отримуємо ОРИГІНАЛЬНИЙ запис з історії (ПОТРІБНА РЕАЛІЗАЦІЯ В Storage)
	originalHistory, err := s.Storage.FindHistoryByID(*originalHistoryID)
	if err != nil || originalHistory == nil {
		log.Printf("ERROR: Failed to fetch original history record %d: %v", *originalHistoryID, err)
		return
	}

	// 3. Визначаємо новий тип, fileID та caption
	newType, newFileID, newCaption := s.extractMediaInfo(msg)

	chatMsg := models.ChatMessage{
		SenderID: c.GetUserID(), // Use internal UserID
		// TgMessageIDSender міститиме TG ID повідомлення, яке потрібно відредагувати/відповісти партнеру (покладаємося на Hub)
		// Нам потрібно лише передати TgMessageIDSender відправника (для оновлення БД)
		TgMessageIDSender: &editedTGID,
		RoomID:            c.GetRoomID(),
		ReplyToMessageID:  originalHistoryID,
	}

	isMediaOriginal := originalHistory.Type != "text"

	// --- A. Перевірка зміни file_id (Медіа) ---
	if isMediaOriginal && newFileID != originalHistory.Content {
		// 1. ФАЙЛ ЗМІНИВСЯ: Відправляємо НОВЕ медіа як відповідь на останнє.
		// Хаб отримає: ReplyToMessageID (на оригінальне повідомлення), Type (photo/video), Content (new file ID).
		chatMsg.Type = newType
		chatMsg.Content = newFileID // Новий file ID
		chatMsg.Metadata = newCaption

		// --- B. Перевірка зміни тексту/caption ---
	} else if isMediaOriginal && newCaption != originalHistory.Metadata {
		// 2. Змінився ЛИШЕ caption медіа
		chatMsg.Type = newType
		chatMsg.Content = originalHistory.Content // Старий fileID (для tg_client, щоб знати, що редагувати Caption)
		chatMsg.Metadata = newCaption             // Новий caption

		//} else if (isMediaOriginal && ) {

	} else if !isMediaOriginal && newCaption != originalHistory.Content {
		// 3. Змінився ЛИШЕ текст текстового повідомлення
		chatMsg.Type = "text"
		chatMsg.Content = newCaption
		chatMsg.Metadata = "" // Це текстове повідомлення

	} else {
		// Нічого не змінилося або не підтримується
		return
	}

	// 4. Надсилаємо в Hub
	s.Hub.IncomingCh <- chatMsg
}

// handleIncomingMessage обробляє нові повідомлення користувачів
func (s *BotService) handleIncomingMessage(msg *tgbotapi.Message) {
	// telegramID := strconv.FormatInt(msg.Chat.ID, 10) // Redundant now

	// getOrCreateClient handles user creation/retrieval
	c := s.getOrCreateClient(msg.Chat.ID)
	if c == nil {
		return // Error logged in getOrCreateClient
	}

	tempID := uint(msg.MessageID)
	chatMsg := models.ChatMessage{
		TgMessageIDSender: &tempID,
		SenderID:          c.GetUserID(), // Use internal UserID
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

// extractMediaInfo уніфіковано витягує тип, fileID та caption з повідомлення
// Потрібно використовувати як msg.*, так і msg.Caption
func (s *BotService) extractMediaInfo(msg *tgbotapi.Message) (msgType string, fileID string, caption string) {
	caption = extractMessageContent(msg) // extractMessageContent бере Text або Caption

	switch {
	case msg.Photo != nil:
		msgType = "photo"
		largestPhoto := msg.Photo[len(msg.Photo)-1]
		fileID = largestPhoto.FileID
	case msg.Video != nil:
		msgType = "video"
		fileID = msg.Video.FileID
	case msg.Animation != nil:
		msgType = "animation"
		fileID = msg.Animation.FileID
	case msg.Sticker != nil:
		msgType = "sticker"
		fileID = msg.Sticker.FileID
	case msg.Voice != nil:
		msgType = "voice"
		fileID = msg.Voice.FileID
	case msg.VideoNote != nil:
		msgType = "video_note"
		fileID = msg.VideoNote.FileID
	default:
		// Якщо немає медіа і є текст/caption (це вже в змінній caption)
		msgType = "text"
		fileID = ""
	}
	return msgType, fileID, caption
}

// Run — головний цикл отримання Telegram-оновлень
func (s *BotService) Run() {
	// Відновлюємо сесії перед запуском обробки оновлень
	s.RestoreActiveSessions()

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
