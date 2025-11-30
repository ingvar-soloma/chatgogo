// Package main is the entry point for the ChatGoGo application.
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

const (
	maxRetries   = 5
	initialDelay = 2 * time.Second
)

// setupDependencies initializes and configures the application's dependencies,
// such as the database and Redis connections. It also runs database migrations.
func setupDependencies() (*gorm.DB, *redis.Client) {
	log.Println("Initializing PostgreSQL connection...")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		dbHost, dbUser, dbPassword, dbName, dbPort)

	var db *gorm.DB
	var err error
	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			log.Println("PostgreSQL connection established.")
			break
		}
		log.Printf("Failed to connect PostgreSQL (Attempt %d/%d). Retrying in %v: %v", i+1, maxRetries, initialDelay, err)
		if i == maxRetries-1 {
			log.Fatalf("Failed to connect PostgreSQL after %d attempts: %v", maxRetries, err)
		}
		time.Sleep(initialDelay)
	}

	log.Println("Initializing Redis connection...")
	redisAddr := fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT"))
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDBStr := os.Getenv("REDIS_DB")
	redisDB := 0
	if redisDBStr != "" {
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

	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		log.Fatalf("Failed to connect Redis at %s: %v", redisAddr, err)
	}

	if err := db.AutoMigrate(&models.ChatRoom{}, &models.User{}, &models.Complaint{}, &models.ChatHistory{}); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("Database and Redis connections established, migrations complete.")
	return db, rdb
}

// main is the application's entry point.
func main() {
	log.Println("Starting ChatGoGo Backend...")
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Error loading .env file")
	}

	db, rdb := setupDependencies()
	s := storage.NewStorageService(db, rdb)

	hub := chathub.NewManagerService(s)
	matcher := chathub.NewMatcherService(hub, s)

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set! Check your .env file or environment variables.")
	}
	botService, err := telegram.NewBotService(botToken, hub, s)
	if err != nil {
		log.Fatalf("Failed to start Telegram bot: %v", err)
	}

	go hub.Run()
	go matcher.Run()
	go botService.Run()

	r := gin.Default()
	h := handler.NewHandler(hub)
	r.GET("/anonid", h.GetAnonID)
	r.GET("/ws", h.ServeWebSocket)

	server := &http.Server{
		Addr:           ":8080",
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(server.ListenAndServe())
}
