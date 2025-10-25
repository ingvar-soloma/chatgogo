package main

import (
	"chatgogo/backend/internal/api/handler"
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/storage"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// Ця функція імітує ініціалізацію DB та Redis (потрібно буде реалізувати)
func setupDependencies() (*gorm.DB, *redis.Client) {
	// NOTE: Реалізуйте тут реальне підключення до PostgreSQL та Redis
	log.Println("Initializing mock DB and Redis connections...")
	return &gorm.DB{}, &redis.Client{} // Повертаємо заглушки для тесту
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
