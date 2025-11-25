package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"zenith-go/internal/auth"
	"zenith-go/internal/config"
	"zenith-go/internal/sankhya"
)

type AuthHandler struct {
	Client  *sankhya.Client
	Config  *config.Config
	Session *auth.SessionManager
}

type loginInput struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DeviceToken string `json:"deviceToken"`
}

// sessionInput foi removido pois o token agora vem no Header

type loginOutput struct {
	Username     string `json:"username"`
	CodUsu       int    `json:"codusu"`
	SessionToken string `json:"sessionToken"`
	SnkSessionID string `json:"snkjsessionid"`
	DeviceToken  string `json:"deviceToken"`
}

// Helper para extrair token do Header Authorization
func getTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	// TIMEOUT: Define limite de 30 segundos para todo o processo de login
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var input loginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	finalDeviceToken := input.DeviceToken
	if finalDeviceToken == "" {
		finalDeviceToken = auth.NewDeviceToken()
	}

	// Repassa 'ctx' para o Client
	codUsuFloat, err := h.Client.VerifyUserAccess(ctx, input.Username)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "Tempo limite de conexão excedido (Timeout)", http.StatusGatewayTimeout)
			return
		}
		if errors.Is(err, sankhya.ErrUserNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else if errors.Is(err, sankhya.ErrUserNotAuthorized) {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	codUsu := int(codUsuFloat)

	// Repassa 'ctx'
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Repassa 'ctx'
	snkJSession, err := h.Client.LoginUser(ctx, input.Username, input.Password)
	if err != nil {
		http.Error(w, "Credenciais inválidas", http.StatusUnauthorized)
		return
	}

	jwtToken, err := auth.GenerateToken(input.Username, codUsu, h.Config.JwtSecret)
	if err != nil {
		http.Error(w, "Erro ao gerar sessão", http.StatusInternalServerError)
		return
	}

	h.Session.Register(jwtToken)

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

	// Token via Header
	token := getTokenFromHeader(r)
	if token == "" {
		http.Error(w, "Token de sessão não fornecido", http.StatusUnauthorized)
		return
	}

	h.Session.Revoke(token)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *AuthHandler) HandleGetPermissions(w http.ResponseWriter, r *http.Request) {
	// TIMEOUT
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodGet && r.Method != http.MethodPost { // Aceita GET também agora, já que não tem body obrigatório
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// Token via Header
	token := getTokenFromHeader(r)
	if token == "" {
		http.Error(w, "Token de sessão não fornecido", http.StatusUnauthorized)
		return
	}

	codUsu, err := auth.ValidateToken(token, h.Config.JwtSecret)
	if err != nil {
		http.Error(w, "Token inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.Session.ValidateAndUpdate(token); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Repassa 'ctx'
	permissions, err := h.Client.GetUserPermissions(ctx, codUsu)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "Tempo limite excedido", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(permissions)
}