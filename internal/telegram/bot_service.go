package telegram

import (
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
	"log"
	"strconv"

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
		if update.Message == nil {
			continue
		}

		// Використовуємо ChatID як унікальний AnonID
		anonID := strconv.FormatInt(update.Message.Chat.ID, 10)

		// 1. Знайти або створити клієнта
		client, exists := s.Hub.Clients[anonID]
		if !exists {
			log.Printf("Реєстрація нового Telegram-клієнта: %s", anonID)
			tgClient := &Client{
				AnonID: anonID,
				RoomID: "",
				Hub:    s.Hub,
				Send:   make(chan models.ChatMessage, 256), // Свій канал
				BotAPI: s.BotAPI,
			}

			s.Hub.RegisterCh <- tgClient // Реєструємо в хабі
			tgClient.Run()               // Запускаємо його writePump
			client = tgClient            // Тепер ми працюємо з ним
		}

		// 2. Конвертуємо повідомлення з Telegram в ChatMessage
		msg := models.ChatMessage{
			SenderID: anonID,
			RoomID:   client.GetRoomID(),
			Content:  update.Message.Text,
		}

		// 3. Визначаємо тип повідомлення (команда чи текст)
		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start", "search":
				msg.Type = "command_search"
			case "stop":
				msg.Type = "command_stop" // Вам треба буде обробити це в ManagerService
			default:
				errMsg := tgbotapi.NewMessage(update.Message.Chat.ID, "Невідома команда.")
				s.BotAPI.Send(errMsg)
				continue
			}
		} else {
			msg.Type = "text"
		}

		// 4. Надсилаємо повідомлення в головний хаб
		// Воно буде оброблене в `case msg := <-m.IncomingCh:`
		s.Hub.IncomingCh <- msg
	}
}
