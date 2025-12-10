package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"zenith-go/internal/auth"
	"zenith-go/internal/notification"
	"zenith-go/internal/sankhya"
)

type TransactionHandler struct {
	Client    *sankhya.Client
	Session   *auth.SessionManager
	JwtSecret string
	Notifier  *notification.EmailService
}

type transactionRequest struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

func getTokenFromHeaderTrans(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

func getHeader(r *http.Request, key string) string {
	return r.Header.Get(key)
}

func (h *TransactionHandler) HandleExecuteTransaction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	bearerToken := getTokenFromHeaderTrans(r)
	snkSessionId := getHeader(r, "Snkjsessionid")

	if bearerToken == "" || snkSessionId == "" {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Tokens ausentes", nil)
		return
	}

	codUsu, err := auth.ValidateToken(bearerToken, h.JwtSecret)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token inválido", err)
		return
	}

	if err := h.Session.ValidateAndUpdate(bearerToken); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Sessão expirada", err)
		return
	}

	var req transactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	input := sankhya.TransactionInput{
		Type:    req.Type,
		Payload: req.Payload,
		CodUsu:  codUsu,
	}

	msg, err := h.Client.ExecuteTransaction(ctx, input, snkSessionId)
	if err != nil {
		// CORREÇÃO: Trata a sessão expirada do Sankhya (Status 3)
		if errors.Is(err, sankhya.ErrUserSessionExpired) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"error":          "Sessão Sankhya expirada. Por favor, faça login novamente.",
				"reauthRequired": true,
			})
			return
		}

		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "permissão") || strings.Contains(err.Error(), "negada") {
			status = http.StatusForbidden
		}
		
		RespondError(w, r, h.Notifier, status, "Falha na transação", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": msg})
}