package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	jwt "github.com/golang-jwt/jwt/v5"
)

// generateJWT генерує JWT з анонімним ID
func generateJWT(anonID string) (string, error) {
	// Встановлюємо claims, включаючи AnonID та термін дії
	claims := jwt.MapClaims{
		"anon_id": anonID,
		"exp":     time.Now().Add(time.Hour * 72).Unix(),
		"iss":     "chatgogo-service", // Видавець
	}

	// Створення нового токена
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Секретний ключ
	var jwtSecret = []byte("YOUR_ULTRA_SECRET_KEY_HERE")

	// SignedString тепер використовує v5 синтаксис
	return token.SignedString(jwtSecret)
}

// GetAnonID створює AnonID та повертає JWT
func (h *Handler) GetAnonID(c *gin.Context) {
	// Генерація унікального анонімного UUID
	anonUUID, _ := uuid.NewRandom()
	anonID := anonUUID.String()

	token, err := generateJWT(anonID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token, "anon_id": anonID})
}
