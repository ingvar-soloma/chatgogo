package handler

import (
	"chatgogo/backend/internal/chathub"

	"github.com/golang-jwt/jwt/v5"
)

// Handler містить посилання на ChatHub
type Handler struct {
	Hub *chathub.ManagerService
}

func NewHandler(hub *chathub.ManagerService) *Handler {
	return &Handler{Hub: hub}
}

// validateAndGetAnonID перевіряє токен та повертає AnonID
func (h *Handler) validateAndGetAnonID(tokenString string) (string, error) {
	// Секретний ключ має бути такий самий, як у generateJWT
	var jwtSecret = []byte("YOUR_ULTRA_SECRET_KEY_HERE")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", jwt.ErrInvalidKey
	}

	anonID, ok := claims["anon_id"].(string)
	if !ok {
		return "", jwt.ErrInvalidKey
	}

	return anonID, nil
}
