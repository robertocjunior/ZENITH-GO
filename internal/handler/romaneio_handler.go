package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
	"zenith-go/internal/auth"
	"zenith-go/internal/config"
	"zenith-go/internal/notification"
	"zenith-go/internal/sankhya"
)

type RomaneioHandler struct {
	Client   *sankhya.Client
	Config   *config.Config
	Session  *auth.SessionManager
	Notifier *notification.EmailService
}

func (h *RomaneioHandler) HandleGetRomaneios(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// Reutiliza a lógica de extração de token do projeto
	token := getTokenFromHeader(r)
	if token == "" {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token ausente", nil)
		return
	}

	// Validação do JWT
	_, _, err := auth.ValidateToken(token, h.Config.JwtSecret)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token inválido", err)
		return
	}

	// Atualiza sessão no Redis (Sliding Expiration)
	if err := h.Session.ValidateAndUpdate(token); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Sessão expirada", err)
		return
	}

	var input sankhya.RomaneioInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	if input.Data == "" {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "O campo 'data' é obrigatório", nil)
		return
	}

	data, err := h.Client.GetRomaneios(ctx, input.Data)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro ao buscar romaneios", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}