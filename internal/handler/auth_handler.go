package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"zenith-go/internal/auth"
	"zenith-go/internal/config"
	"zenith-go/internal/notification" // Import
	"zenith-go/internal/sankhya"
)

type AuthHandler struct {
	Client   *sankhya.Client
	Config   *config.Config
	Session  *auth.SessionManager
	Notifier *notification.EmailService // Injeção
}

type loginInput struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DeviceToken string `json:"deviceToken"`
}

type loginOutput struct {
	Username     string `json:"username"`
	CodUsu       int    `json:"codusu"`
	SessionToken string `json:"sessionToken"`
	SnkSessionID string `json:"snkjsessionid"`
	DeviceToken  string `json:"deviceToken"`
}

func getTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var input loginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	slog.Info("Tentativa de login", "username", input.Username, "ip", r.RemoteAddr)

	finalDeviceToken := input.DeviceToken
	if finalDeviceToken == "" {
		finalDeviceToken = auth.NewDeviceToken()
	}

	// 1. Verificar Usuário
	codUsuFloat, err := h.Client.VerifyUserAccess(ctx, input.Username)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			RespondError(w, r, h.Notifier, http.StatusGatewayTimeout, "Timeout no login", err)
			return
		}
		if errors.Is(err, sankhya.ErrUserNotFound) {
			RespondError(w, r, h.Notifier, http.StatusNotFound, err.Error(), nil)
		} else if errors.Is(err, sankhya.ErrUserNotAuthorized) {
			RespondError(w, r, h.Notifier, http.StatusForbidden, err.Error(), nil)
		} else {
			RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro interno ao verificar usuário", err)
		}
		return
	}
	codUsu := int(codUsuFloat)

	// 2. Verificar Dispositivo
	if err := h.Client.VerifyDevice(ctx, codUsu, finalDeviceToken); err != nil {
		if errors.Is(err, sankhya.ErrDevicePendingApproval) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error":       err.Error(),
				"deviceToken": finalDeviceToken,
			})
			return
		}
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro ao verificar dispositivo", err)
		return
	}

	// 3. Login no Sankhya
	snkJSession, err := h.Client.LoginUser(ctx, input.Username, input.Password)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Credenciais inválidas no ERP", err)
		return
	}

	// 4. JWT
	jwtToken, err := auth.GenerateToken(input.Username, codUsu, h.Config.JwtSecret)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro ao gerar JWT", err)
		return
	}

	// 5. Redis (ATUALIZADO: Passando snkJSession)
	if err := h.Session.Register(jwtToken, snkJSession); err != nil {
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro ao salvar sessão", err)
		return
	}

	slog.Info("Login realizado com sucesso", "username", input.Username)

	response := loginOutput{
		Username:     input.Username,
		CodUsu:       codUsu,
		SessionToken: jwtToken,
		SnkSessionID: snkJSession,
		DeviceToken:  finalDeviceToken,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}
	token := getTokenFromHeader(r)
	if token == "" {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token ausente", nil)
		return
	}
	h.Session.Revoke(token)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *AuthHandler) HandleGetPermissions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	token := getTokenFromHeader(r)
	if token == "" {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token ausente", nil)
		return
	}

	codUsu, err := auth.ValidateToken(token, h.Config.JwtSecret)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token inválido", err)
		return
	}

	if err := h.Session.ValidateAndUpdate(token); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Sessão expirada", err)
		return
	}

	permissions, err := h.Client.GetUserPermissions(ctx, codUsu)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro ao buscar permissões", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(permissions)
}