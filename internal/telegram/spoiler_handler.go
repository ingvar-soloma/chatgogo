package telegram

import (
	"chatgogo/backend/internal/models"
	"context"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SpoilerStorage defines the storage methods required by the spoiler handler.
// This allows the handler to be tested and compiled without modifying the main Storage interface immediately.
type SpoilerStorage interface {
	SaveUserIfNotExists(telegramID int64) (*models.User, error)
	UpdateUserMediaSpoiler(userID string, value bool) error
}

// HandleSpoilerCommand processes /spoiler_on and /spoiler_off commands.
// It updates the user's preference in the storage and sends a confirmation message.
func HandleSpoilerCommand(ctx context.Context, update *tgbotapi.Update, s SpoilerStorage, bot *tgbotapi.BotAPI) {
	if update.Message == nil {
		return
	}

	command := update.Message.Command()
	var enableSpoiler bool
	var responseText string

	switch command {
	case "spoiler_on":
		enableSpoiler = true
		responseText = "Default media spoiler enabled. Your photos and videos will now be covered by a spoiler."
	case "spoiler_off":
		enableSpoiler = false
		responseText = "Default media spoiler disabled. Your photos and videos will be visible immediately."
	default:
		return
	}

	// Ensure user exists and get their internal ID
	user, err := s.SaveUserIfNotExists(update.Message.From.ID)
	if err != nil {
		log.Printf("Error retrieving user for spoiler command: %v", err)
		responseText = "An error occurred while processing your request."
	} else {
		// Update the preference
		if err := s.UpdateUserMediaSpoiler(user.ID, enableSpoiler); err != nil {
			log.Printf("Error updating spoiler preference for user %s: %v", user.ID, err)
			responseText = "Failed to update your preference. Please try again later."
		}
	}

	// Send confirmation
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, responseText)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Error sending spoiler confirmation: %v", err)
	}
}
