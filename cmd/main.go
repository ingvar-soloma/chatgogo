package main

import (
	"chatgogo/backend/internal/api/handler"
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
	"chatgogo/backend/internal/storage"
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Ця функція імітує ініціалізацію DB та Redis (потрібно буде реалізувати)
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

	// 1. Ініціалізація залежностей
	db, rdb := setupDependencies()
	s := storage.NewStorageService(db, rdb)

	// 2. Ініціалізація Chat Hub та Matcher
	hub := chathub.NewManagerService(s)
	matcher := chathub.NewMatcherService(hub, s)

	// 3. Запуск основних Goroutines
	go hub.Run()     // Головний диспетчер
	go matcher.Run() // Сервіс пошуку

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
