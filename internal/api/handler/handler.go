package handler

import "chatgogo/backend/internal/chathub"

// Handler містить посилання на ChatHub
type Handler struct {
	Hub *chathub.ManagerService
}

func NewHandler(hub *chathub.ManagerService) *Handler {
	return &Handler{Hub: hub}
}
