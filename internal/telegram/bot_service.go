// Package telegram handles the integration with the Telegram Bot API.
// It is responsible for receiving updates from Telegram, processing them,
// and communicating with the central chat hub.
package telegram

import (
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/localization"
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// BotService is responsible for receiving Telegram updates and routing them to the hub.
type BotService struct {
	BotAPI    *tgbotapi.BotAPI
	Hub       *chathub.ManagerService
	Storage   storage.Storage
	Localizer *localization.Localizer
}

// NewBotService creates a new BotService instance.
func NewBotService(token string, hub *chathub.ManagerService, s storage.Storage) (*BotService, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	bot.Debug = false
	log.Printf("✅ Authorized on account %s", bot.Self.UserName)

	localizer, err := localization.NewLocalizer("internal/localization")
	if err != nil {
		return nil, fmt.Errorf("failed to create localizer: %w", err)
	}

	return &BotService{BotAPI: bot, Hub: hub, Storage: s, Localizer: localizer}, nil
}

// extractMessageContent uniformly extracts text or a caption from a message.
func extractMessageContent(msg *tgbotapi.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Text != "" {
		return msg.Text
	}
	return msg.Caption
}

// getOrCreateClient retrieves an existing Telegram client or creates a new one.
func (s *BotService) getOrCreateClient(chatID int64) *Client {
	user, err := s.Storage.SaveUserIfNotExists(chatID)
	if err != nil {
		log.Printf("FATAL: Failed to get/create user for TelegramID %s: %v", chatID, err)
		return nil
	}
	userID := user.ID

	if existingClient, ok := s.Hub.Clients[userID]; ok {
		if client, ok := existingClient.(*Client); ok {
			return client
		}
		log.Printf("ERROR: Client %s (User: %s) is not of type *telegram.Client", chatID, userID)
	}

	newClient := &Client{
		UserID:    userID,
		AnonID:    chatID,
		Hub:       s.Hub,
		Send:      make(chan models.ChatMessage, 10),
		BotAPI:    s.BotAPI,
		Storage:   s.Storage,
		Localizer: s.Localizer,
	}

	activeRoomID, err := s.Storage.GetActiveRoomIDForUser(userID)
	if err == nil && activeRoomID != "" {
		newClient.SetRoomID(activeRoomID)
		log.Printf("Client %s (User: %s) restored to room %s synchronously.", chatID, userID, activeRoomID)
	}

	s.Hub.RegisterCh <- newClient
	go newClient.Run()
	return newClient
}

// RestoreActiveSessions restores sessions for users who are in active chat rooms.
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
		restoreUser := func(userIDStr string) {
			// userIDStr is the internal UUID, not the Telegram ID.
			// We need to look up the user to get their Telegram ID.
			user, err := s.Storage.GetUserByID(userIDStr)
			if err != nil {
				log.Printf("Failed to find user %s for restoration: %v", userIDStr, err)
				return
			}
			if user.TelegramID == 0 {
				log.Printf("User %s has no Telegram ID, cannot restore session", userIDStr)
				return
			}

			chatID := user.TelegramID
			s.getOrCreateClient(chatID)
		}
		restoreUser(room.User1ID)
		restoreUser(room.User2ID)
	}
	log.Println("Active Telegram sessions restored.")
}

// handleEditedMessage processes edited messages.
func (s *BotService) handleEditedMessage(msg *tgbotapi.Message) {
	c := s.getOrCreateClient(msg.Chat.ID)
	editedTGID := uint(msg.MessageID)
	originalHistoryID, err := s.Storage.FindOriginalHistoryIDByTgIDMedia(editedTGID)
	if err != nil || originalHistoryID == nil {
		log.Printf("ERROR/WARN: Ignoring edit for un-tracked TG ID %d: %v", editedTGID, err)
		return
	}

	originalHistory, err := s.Storage.FindHistoryByID(*originalHistoryID)
	if err != nil || originalHistory == nil {
		log.Printf("ERROR: Failed to fetch original history record %d: %v", *originalHistoryID, err)
		return
	}

	newType, newFileID, newCaption := s.extractMediaInfo(msg)
	chatMsg := models.ChatMessage{
		SenderID:          c.GetUserID(),
		TgMessageIDSender: &editedTGID,
		RoomID:            c.GetRoomID(),
		ReplyToMessageID:  originalHistoryID,
	}

	isMediaOriginal := originalHistory.Type != "text"
	if isMediaOriginal && newFileID != originalHistory.Content {
		chatMsg.Type = newType
		chatMsg.Content = newFileID
		chatMsg.Metadata = newCaption
	} else if isMediaOriginal && newCaption != originalHistory.Metadata {
		chatMsg.Type = newType
		chatMsg.Content = originalHistory.Content
		chatMsg.Metadata = newCaption
	} else if !isMediaOriginal && newCaption != originalHistory.Content {
		chatMsg.Type = "text"
		chatMsg.Content = newCaption
	} else {
		return
	}
	s.Hub.IncomingCh <- chatMsg
}

// handleIncomingMessage processes new messages from users.
func (s *BotService) handleIncomingMessage(msg *tgbotapi.Message) {
	c := s.getOrCreateClient(msg.Chat.ID)
	if c == nil {
		return
	}

	tempID := uint(msg.MessageID)
	chatMsg := models.ChatMessage{
		TgMessageIDSender: &tempID,
		SenderID:          c.GetUserID(),
		RoomID:            c.GetRoomID(),
	}

	if msg.ReplyToMessage != nil {
		replyTGID := uint(msg.ReplyToMessage.MessageID)
		if originalHistoryID, err := s.Storage.FindOriginalHistoryIDByTgID(replyTGID); err == nil && originalHistoryID != nil {
			chatMsg.ReplyToMessageID = originalHistoryID
		}
	}

	switch {
	case msg.Text != "":
		chatMsg.Content = msg.Text
		s.handleTextMessage(&chatMsg)
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
		user, err := s.Storage.GetUserByTelegramID(msg.Chat.ID)
		if err != nil {
			log.Printf("Error getting user by telegram id: %v", err)
			return
		}

		unsupportedMsg := tgbotapi.NewMessage(msg.Chat.ID, s.Localizer.GetString(user.Language, "unsupported_message_type"))
		s.BotAPI.Send(unsupportedMsg)
		return
	}

	if chatMsg.RoomID == "" && !strings.HasPrefix(chatMsg.Type, "command_") {
		user, err := s.Storage.GetUserByTelegramID(msg.Chat.ID)
		if err != nil {
			log.Printf("Error getting user by telegram id: %v", err)
			return
		}
		c.GetSendChannel() <- models.ChatMessage{
			Type:    "system_info",
			Content: s.Localizer.GetString(user.Language, "not_in_chat"),
		}
		return
	}

	s.Hub.IncomingCh <- chatMsg
}

// handleTextMessage processes text messages and commands.
func (s *BotService) handleTextMessage(chatMsg *models.ChatMessage) {
	chatMsg.Type = "text"
	if !strings.HasPrefix(chatMsg.Content, "/") {
		return
	}

	parts := strings.SplitN(chatMsg.Content, " ", 2)
	command := strings.TrimPrefix(parts[0], "/")

	switch command {
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
		chatMsg.Type = "unknown_command"
	}
}

// handleLanguageCommand sends a message with a keyboard to choose a language.
func (s *BotService) handleLanguageCommand(chatID int64) {
	user, err := s.Storage.GetUserByTelegramID(chatID)
	if err != nil {
		log.Printf("Error getting user by telegram id: %v", err)
		return
	}

	msg := tgbotapi.NewMessage(chatID, s.Localizer.GetString(user.Language, "choose_language"))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("English", "set_lang_en"),
			tgbotapi.NewInlineKeyboardButtonData("Русский", "set_lang_ru"),
			tgbotapi.NewInlineKeyboardButtonData("Українська", "set_lang_ua"),
		),
	)
	s.BotAPI.Send(msg)
}

// extractMediaInfo uniformly extracts media type, file ID, and caption from a message.
func (s *BotService) extractMediaInfo(msg *tgbotapi.Message) (msgType, fileID, caption string) {
	caption = extractMessageContent(msg)
	switch {
	case msg.Photo != nil:
		msgType = "photo"
		fileID = msg.Photo[len(msg.Photo)-1].FileID
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
		msgType = "text"
	}
	return
}

// Run is the main loop for receiving Telegram updates.
func (s *BotService) Run() {
	s.RestoreActiveSessions()
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := s.BotAPI.GetUpdatesChan(u)

	for update := range updates {
		switch {
		case update.EditedMessage != nil:
			s.handleEditedMessage(update.EditedMessage)
		case update.Message != nil:
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "language":
					s.handleLanguageCommand(update.Message.Chat.ID)
					continue
				case "spoiler_on", "spoiler_off":
					HandleSpoilerCommand(context.Background(), &update, s.Storage, s.BotAPI)
					continue
				}
			}
			s.handleIncomingMessage(update.Message)
		case update.CallbackQuery != nil:
			s.handleCallbackQuery(update.CallbackQuery)
		}
	}
}

func (s *BotService) handleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery) {
	// Respond to the callback query to remove the "loading" state
	callback := tgbotapi.NewCallback(callbackQuery.ID, "")
	if _, err := s.BotAPI.Request(callback); err != nil {
		log.Printf("failed to send callback response: %v", err)
	}

	// Extract language code from data
	langCode := strings.TrimPrefix(callbackQuery.Data, "set_lang_")
	chatID := callbackQuery.Message.Chat.ID

	// Update user's language in the database
	err := s.Storage.UpdateUserLanguage(chatID, langCode)
	if err != nil {
		log.Printf("failed to update user language: %v", err)
		return
	}

	// Send a confirmation message
	user, err := s.Storage.GetUserByTelegramID(chatID)
	if err != nil {
		log.Printf("Error getting user by telegram id: %v", err)
		return
	}

	msg := tgbotapi.NewMessage(chatID, s.Localizer.GetString(user.Language, "language_changed"))
	s.BotAPI.Send(msg)
}
