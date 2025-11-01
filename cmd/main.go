package main

import (
	"chatgogo/backend/internal/api/handler"
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"chatgogo/backend/internal/telegram"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const maxRetries = 5                 // Максимальна кількість спроб підключення
const initialDelay = 2 * time.Second // Затримка між спробами

func setupDependencies() (*gorm.DB, *redis.Client) {
	// 1. PostgreSQL
	log.Println("Initializing PostgreSQL connection...")

	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		dbHost, dbUser, dbPassword, dbName, dbPort)

	// FIX: Declare db and err outside the loop scope
	var db *gorm.DB
	var err error

	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			// Успішне підключення
			log.Println("PostgreSQL connection established.")
			break
		}

		log.Printf("Failed to connect PostgreSQL (Attempt %d/%d). Retrying in %v: %v", i+1, maxRetries, initialDelay, err)

		if i == maxRetries-1 {
			// Якщо це була остання спроба, зупиняємо додаток
			log.Fatalf("Failed to connect PostgreSQL after %d attempts: %v", maxRetries, err)
		}

		time.Sleep(initialDelay)
	}

	// 2. Redis
	log.Println("Initializing Redis connection...")

	redisAddr := os.Getenv("REDIS_ADDR")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDBStr := os.Getenv("REDIS_DB")
	redisDB := 0 // За замовчуванням

	if redisDBStr != "" {
		// Конвертуємо номер бази даних з рядка в int
		var parseErr error
		redisDB, parseErr = strconv.Atoi(redisDBStr)
		if parseErr != nil {
			log.Printf("Warning: Invalid REDIS_DB value '%s'. Using default DB 0.", redisDBStr)
			redisDB = 0
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	// Перевірка з'єднання Redis
	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Fatalf("Failed to connect Redis at %s: %v", redisAddr, err)
	}

	// 3. Міграції (Створення таблиць)
	err = db.AutoMigrate(
		&models.ChatRoom{},
		&models.User{},
		&models.Complaint{},
		&models.ChatHistory{},
	)
	if err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("Database and Redis connections established, migrations complete.")
	return db, rdb
}

func main() {
	log.Println("Starting ChatGoGo Backend...")

	// Завантаження змінних середовища з .env
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

	// Telegram Bot Token
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN не встановлено! Перевірте файл .env або змінні середовища.")
	}
	botService, err := telegram.NewBotService(botToken, hub, s)
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
	server := &http.Server{
		Addr:           ":8080",
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Fatal(server.ListenAndServe())
}
