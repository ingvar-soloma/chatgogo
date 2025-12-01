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
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	StateWaitingForAge       = "waiting_for_age"
	StateWaitingForInterests = "waiting_for_interests"
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
		log.Printf("FATAL: Failed to get/create user for TelegramID %d: %v", chatID, err)
		return nil
	}
	userID := user.ID

	if existingClient, ok := s.Hub.Clients[userID]; ok {
		if client, ok := existingClient.(*Client); ok {
			return client
		}
		log.Printf("ERROR: Client %d (User: %s) is not of type *telegram.Client", chatID, userID)
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
		log.Printf("Client %d (User: %s) restored to room %s synchronously.", chatID, userID, activeRoomID)
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
	case "block":
		chatMsg.Type = "command_block"
	case "unblock":
		chatMsg.Type = "command_unblock"
	case "profile":
		// We need to handle this differently because we don't have the chatID here directly in a convenient way
		// if we want to call handleProfileCommand.
		// However, handleIncomingMessage calls this.
		// Let's adjust handleIncomingMessage to handle commands that don't need to go to the hub.
		// Actually, for now, let's just mark it as command_profile and handle it in the switch in handleIncomingMessage?
		// No, handleTextMessage is called by handleIncomingMessage.
		// Let's just return here and let the caller handle it?
		// Or better, let's pass the bot service reference or just handle it here if we can.
		// But we don't have the chatID easily accessible as int64 here, only as string in SenderID? No.
		// The caller has msg.Chat.ID.
		chatMsg.Type = "command_profile"
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
				case "profile":
					s.handleProfileCommand(update.Message.Chat.ID)
					continue
				}
			}
			s.handleIncomingMessage(update.Message)
		case update.CallbackQuery != nil:
			if strings.HasPrefix(update.CallbackQuery.Data, "edit_") || strings.HasPrefix(update.CallbackQuery.Data, "set_gender_") {
				s.handleProfileCallback(update.CallbackQuery)
			} else {
				s.handleCallbackQuery(update.CallbackQuery)
			}
		}
	}
}

func (s *BotService) handleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery) {
	// Respond to the callback query to remove the "loading" state
	callback := tgbotapi.NewCallback(callbackQuery.ID, "")
	if _, err := s.BotAPI.Request(callback); err != nil {
		log.Printf("failed to send callback response: %v", err)
	}

	chatID := callbackQuery.Message.Chat.ID
	user, err := s.Storage.GetUserByTelegramID(chatID)
	if err != nil {
		log.Printf("Error getting user by telegram id: %v", err)
		return
	}

	if strings.HasPrefix(callbackQuery.Data, "unblock_") {
		blockedUserID := strings.TrimPrefix(callbackQuery.Data, "unblock_")
		err := s.Storage.UnblockUser(user.ID, blockedUserID)
		if err != nil {
			log.Printf("Error unblocking user: %v", err)
			return
		}

		msg := tgbotapi.NewMessage(chatID, s.Localizer.GetString(user.Language, "user_unblocked"))
		s.BotAPI.Send(msg)
		return
	}

	// Extract language code from data
	langCode := strings.TrimPrefix(callbackQuery.Data, "set_lang_")

	// Update user's language in the database
	err = s.Storage.UpdateUserLanguage(chatID, langCode)
	if err != nil {
		log.Printf("failed to update user language: %v", err)
		return
	}

	// Send a confirmation message
	user, err = s.Storage.GetUserByTelegramID(chatID)
	if err != nil {
		log.Printf("Error getting user by telegram id: %v", err)
		return
	}

	msg := tgbotapi.NewMessage(chatID, s.Localizer.GetString(user.Language, "language_changed"))
	s.BotAPI.Send(msg)
}

// handleProfileCommand sends the user's profile information and edit options.
func (s *BotService) handleProfileCommand(chatID int64) {
	user, err := s.Storage.GetUserByTelegramID(chatID)
	if err != nil {
		log.Printf("Error getting user by telegram id: %v", err)
		return
	}

	// Format interests
	interestsStr := "None"
	if len(user.Interests) > 0 {
		interestsStr = strings.Join(user.Interests, ", ")
	}

	// Format gender
	genderStr := "Not specified"
	if user.Gender != "" {
		if user.Gender == "male" {
			genderStr = s.Localizer.GetString(user.Language, "gender_male")
		} else if user.Gender == "female" {
			genderStr = s.Localizer.GetString(user.Language, "gender_female")
		} else {
			genderStr = user.Gender
		}
	}

	profileText := fmt.Sprintf(s.Localizer.GetString(user.Language, "profile_view"),
		user.Age, genderStr, interestsStr, user.RatingScore)

	msg := tgbotapi.NewMessage(chatID, profileText)
	msg.ParseMode = tgbotapi.ModeMarkdown

	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(s.Localizer.GetString(user.Language, "btn_edit_age"), "edit_age"),
			tgbotapi.NewInlineKeyboardButtonData(s.Localizer.GetString(user.Language, "btn_edit_gender"), "edit_gender"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(s.Localizer.GetString(user.Language, "btn_edit_interests"), "edit_interests"),
		),
	)
}

// deleteMessage deletes a message from the chat.
func (s *BotService) deleteMessage(chatID int64, messageID int) {
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
	if _, err := s.BotAPI.Request(deleteMsg); err != nil {
		log.Printf("Failed to delete message %d in chat %d: %v", messageID, chatID, err)
	}
}

// handleProfileCallback handles callback queries related to profile editing.
func (s *BotService) handleProfileCallback(callbackQuery *tgbotapi.CallbackQuery) {
	chatID := callbackQuery.Message.Chat.ID
	user, err := s.Storage.GetUserByTelegramID(chatID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		return
	}

	// Answer the callback query to stop the loading animation
	callback := tgbotapi.NewCallback(callbackQuery.ID, "")
	s.BotAPI.Request(callback)

	switch callbackQuery.Data {
	case "edit_age":
		s.Storage.SetUserState(user.ID, StateWaitingForAge)
		msg := tgbotapi.NewMessage(chatID, s.Localizer.GetString(user.Language, "prompt_age"))
		sentMsg, _ := s.BotAPI.Send(msg)
		s.Storage.SetUserAttribute(user.ID, "last_prompt_msg_id", strconv.Itoa(sentMsg.MessageID))

	case "edit_gender":
		msg := tgbotapi.NewMessage(chatID, s.Localizer.GetString(user.Language, "choose_gender"))
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(s.Localizer.GetString(user.Language, "gender_male"), "set_gender_male"),
				tgbotapi.NewInlineKeyboardButtonData(s.Localizer.GetString(user.Language, "gender_female"), "set_gender_female"),
			),
		)
		s.BotAPI.Send(msg)

	case "edit_interests":
		s.Storage.SetUserState(user.ID, StateWaitingForInterests)
		msg := tgbotapi.NewMessage(chatID, s.Localizer.GetString(user.Language, "prompt_interests"))
		sentMsg, _ := s.BotAPI.Send(msg)
		s.Storage.SetUserAttribute(user.ID, "last_prompt_msg_id", strconv.Itoa(sentMsg.MessageID))

	case "set_gender_male":
		s.Storage.UpdateUserGender(user.ID, "male")
		s.handleProfileCommand(chatID)

	case "set_gender_female":
		s.Storage.UpdateUserGender(user.ID, "female")
		s.handleProfileCommand(chatID)
	}
}

// handleBlockCommand handles the /block command.
func (s *BotService) handleBlockCommand(chatID int64) {
	user, err := s.Storage.GetUserByTelegramID(chatID)
	if err != nil {
		log.Printf("Error getting user by telegram id: %v", err)
		return
	}

	roomID, err := s.Storage.GetActiveRoomIDForUser(user.ID)
	if err != nil {
		log.Printf("Error getting active room: %v", err)
		// Fall through to check for last partner
	}

	var partnerID string
	if roomID != "" {
		room, err := s.Storage.GetRoomByID(roomID)
		if err != nil {
			log.Printf("Error getting room by id: %v", err)
			return
		}

		if room.User1ID == user.ID {
			partnerID = room.User2ID
		} else {
			partnerID = room.User1ID
		}
	} else {
		lastPartnerID, err := s.Storage.GetUserAttribute(user.ID, "last_partner_id")
		if err != nil || lastPartnerID == "" {
			msg := tgbotapi.NewMessage(chatID, s.Localizer.GetString(user.Language, "no_one_to_block"))
			s.BotAPI.Send(msg)
			return
		}
		partnerID = lastPartnerID
	}

	err = s.Storage.BlockUser(user.ID, partnerID)
	if err != nil {
		log.Printf("Error blocking user: %v", err)
		return
	}

	msg := tgbotapi.NewMessage(chatID, s.Localizer.GetString(user.Language, "user_blocked"))
	s.BotAPI.Send(msg)

	// Disconnect the chat
	s.Hub.IncomingCh <- models.ChatMessage{
		SenderID: user.ID,
		RoomID:   roomID,
		Type:     "command_stop",
	}
}

// handleUnblockCommand handles the /unblock command.
func (s *BotService) handleUnblockCommand(chatID int64) {
	user, err := s.Storage.GetUserByTelegramID(chatID)
	if err != nil {
		log.Printf("Error getting user by telegram id: %v", err)
		return
	}

	if len(user.BlockedUsers) == 0 {
		msg := tgbotapi.NewMessage(chatID, s.Localizer.GetString(user.Language, "no_blocked_users"))
		s.BotAPI.Send(msg)
		return
	}

	var buttons [][]tgbotapi.InlineKeyboardButton
	for i, blockedUserID := range user.BlockedUsers {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Blocked User %d", i+1), fmt.Sprintf("unblock_%s", blockedUserID)),
		))
	}

	msg := tgbotapi.NewMessage(chatID, s.Localizer.GetString(user.Language, "select_user_to_unblock"))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	s.BotAPI.Send(msg)
}

func (s *BotService) handleIncomingMessage(msg *tgbotapi.Message) {
	c := s.getOrCreateClient(msg.Chat.ID)
	if c == nil {
		return
	}

	// Fetch user to get language
	user, err := s.Storage.GetUserByTelegramID(msg.Chat.ID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		return
	}

	// Check for active user state (e.g. waiting for age/interests)
	userState, err := s.Storage.GetUserState(c.UserID)
	if err == nil && userState != "" {
		// Delete user's input message
		s.deleteMessage(msg.Chat.ID, msg.MessageID)

		// Delete the previous prompt message
		lastPromptIDStr, _ := s.Storage.GetUserAttribute(c.UserID, "last_prompt_msg_id")
		if lastPromptIDStr != "" {
			if lastPromptID, err := strconv.Atoi(lastPromptIDStr); err == nil {
				s.deleteMessage(msg.Chat.ID, lastPromptID)
			}
			s.Storage.DeleteUserAttribute(c.UserID, "last_prompt_msg_id")
		}

		switch userState {
		case StateWaitingForAge:
			age, err := strconv.Atoi(msg.Text)
			if err != nil || age < 10 || age > 100 {
				errMsg := tgbotapi.NewMessage(msg.Chat.ID, s.Localizer.GetString(user.Language, "invalid_age"))
				sentMsg, _ := s.BotAPI.Send(errMsg)
				s.Storage.SetUserAttribute(c.UserID, "last_prompt_msg_id", strconv.Itoa(sentMsg.MessageID))
				return
			}
			s.Storage.UpdateUserAge(c.UserID, age)
			s.Storage.ClearUserState(c.UserID)
			s.handleProfileCommand(msg.Chat.ID)
			return

		case StateWaitingForInterests:
			interests := strings.Split(msg.Text, ",")
			cleanInterests := make([]string, 0)
			for _, i := range interests {
				trimmed := strings.TrimSpace(i)
				if trimmed != "" {
					cleanInterests = append(cleanInterests, trimmed)
				}
			}

			if len(cleanInterests) == 0 {
				errMsg := tgbotapi.NewMessage(msg.Chat.ID, s.Localizer.GetString(user.Language, "invalid_interests"))
				sentMsg, _ := s.BotAPI.Send(errMsg)
				s.Storage.SetUserAttribute(c.UserID, "last_prompt_msg_id", strconv.Itoa(sentMsg.MessageID))
				return
			}

			s.Storage.UpdateUserInterests(c.UserID, cleanInterests)
			s.Storage.ClearUserState(c.UserID)
			s.handleProfileCommand(msg.Chat.ID)
			return
		}
	}

	// Handle commands
	if msg.IsCommand() {
		chatMsg := models.ChatMessage{
			SenderID: c.UserID,
			RoomID:   c.RoomID,
			Content:  msg.Text,
			Type:     "text",
		}
		s.handleTextMessage(&chatMsg) // This will set chatMsg.Type to command_...
		switch chatMsg.Type {
		case "command_profile":
			s.handleProfileCommand(msg.Chat.ID)
			return
		case "command_block":
			s.handleBlockCommand(msg.Chat.ID)
			return
		case "command_unblock":
			s.handleUnblockCommand(msg.Chat.ID)
			return
		default:
			s.Hub.IncomingCh <- chatMsg
		}
		return
	}

	// Handle regular messages
	msgType, fileID, caption := s.extractMediaInfo(msg)

	content := caption
	metadata := ""
	if msgType != "text" {
		content = fileID
		metadata = caption
	}

	chatMsg := models.ChatMessage{
		SenderID: c.UserID,
		RoomID:   c.RoomID,
		Type:     msgType,
		Content:  content,
		Metadata: metadata,
	}

	s.Hub.IncomingCh <- chatMsg
}
