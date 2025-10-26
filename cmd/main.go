package main

import (
	"chatgogo/backend/internal/api/handler"
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"chatgogo/backend/internal/telegram"
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupDependencies() (*gorm.DB, *redis.Client) {
	// 1. PostgreSQL (Використовуємо дані з docker-compose)
	dsn := "host=localhost user=user password=password dbname=chatgogodb port=5432 sslmode=disable"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect PostgreSQL: %v", err)
	}

	// 2. Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})

	// Перевірка з'єднання Redis
	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Fatalf("Failed to connect Redis: %v", err)
	}

	// 3. Міграції (Створення таблиць)
	// ВАЖЛИВО: Потрібно імпортувати всі моделі з internal/models
	err = db.AutoMigrate(
		&models.ChatRoom{},
		&models.User{}, // Додайте всі моделі, які ви використовуєте
		// &models.Complaint{},
	)
	if err != nil {
		// Якщо міграція не спрацювала, зупиняємо додаток
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("Database and Redis connections established, migrations complete.")
	return db, rdb
}

func main() {
	log.Println("Starting ChatGoGo Backend...")

	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file")
	}

	// 1. Ініціалізація залежностей
	db, rdb := setupDependencies()
	s := storage.NewStorageService(db, rdb)

	// 2. Ініціалізація Chat Hub та Matcher
	hub := chathub.NewManagerService(s)
	matcher := chathub.NewMatcherService(hub, s)

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN не встановлено!")
	}
	botService, err := telegram.NewBotService(botToken, hub)
	if err != nil {
		log.Fatalf("Не вдалося запустити Telegram-бота: %v", err)
	}

	// 3. Запуск основних Goroutines
	go hub.Run()        // Головний диспетчер
	go matcher.Run()    // Сервіс пошуку
	go botService.Run() // tg bot service

	// 4. Налаштування Gin та роутингу
	r := gin.Default()
	h := handler.NewHandler(hub)

	// Роути
	r.GET("/anonid", h.GetAnonID)  // Отримання JWT для AnonID
	r.GET("/ws", h.ServeWebSocket) // WebSocket Upgrade

	// Запуск HTTP-сервера
	server := &http.Server{ // <--- Змінили s на server
		Addr:           ":8080",
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Fatal(server.ListenAndServe())
}
