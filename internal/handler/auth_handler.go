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
	// Contexto com timeout para o fluxo de login
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var input loginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		slog.Warn("Login falhou: JSON inválido", "ip", r.RemoteAddr)
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	slog.Info("Tentativa de login", "username", input.Username, "ip", r.RemoteAddr)

	finalDeviceToken := input.DeviceToken
	if finalDeviceToken == "" {
		finalDeviceToken = auth.NewDeviceToken()
		slog.Debug("DeviceToken não informado, gerado novo", "token", finalDeviceToken)
	}

	// 1. Verificar Usuário e Permissão Básica
	codUsuFloat, err := h.Client.VerifyUserAccess(ctx, input.Username)
	if err != nil {
		slog.Warn("Falha ao verificar usuário", "username", input.Username, "error", err)
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "Timeout no login", http.StatusGatewayTimeout)
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

	// 2. Verificar/Registrar Dispositivo
	if err := h.Client.VerifyDevice(ctx, codUsu, finalDeviceToken); err != nil {
		slog.Warn("Dispositivo não autorizado", "username", input.Username, "device", finalDeviceToken, "error", err)
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

	// 3. Login no Sankhya (Obter JSESSIONID)
	snkJSession, err := h.Client.LoginUser(ctx, input.Username, input.Password)
	if err != nil {
		slog.Warn("Senha inválida no Sankhya", "username", input.Username)
		http.Error(w, "Credenciais inválidas", http.StatusUnauthorized)
		return
	}

	// 4. Gerar JWT da API
	jwtToken, err := auth.GenerateToken(input.Username, codUsu, h.Config.JwtSecret)
	if err != nil {
		slog.Error("Erro ao gerar JWT", "error", err)
		http.Error(w, "Erro ao gerar sessão", http.StatusInternalServerError)
		return
	}

	// 5. Registrar Sessão no Redis
	if err := h.Session.Register(jwtToken); err != nil {
		slog.Error("Erro ao salvar sessão no Redis", "error", err)
		http.Error(w, "Erro interno de sessão", http.StatusInternalServerError)
		return
	}

	slog.Info("Login realizado com sucesso", "username", input.Username, "codusu", codUsu)

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
		slog.Warn("Logout tentado sem token")
		http.Error(w, "Token de sessão não fornecido", http.StatusUnauthorized)
		return
	}

	// Remove do Redis
	h.Session.Revoke(token)
	
	// Log seguro (apenas prefixo do token)
	tokenPrefix := ""
	if len(token) > 10 {
		tokenPrefix = token[:10] + "..."
	}
	slog.Info("Logout realizado", "token_prefix", tokenPrefix)

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
		http.Error(w, "Token de sessão não fornecido", http.StatusUnauthorized)
		return
	}

	// Valida JWT (Assinatura)
	codUsu, err := auth.ValidateToken(token, h.Config.JwtSecret)
	if err != nil {
		slog.Warn("Permissões negadas: token inválido", "error", err)
		http.Error(w, "Token inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Valida Sessão no Redis
	if err := h.Session.ValidateAndUpdate(token); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	slog.Debug("Buscando permissões", "codusu", codUsu)

	permissions, err := h.Client.GetUserPermissions(ctx, codUsu)
	if err != nil {
		slog.Error("Erro ao buscar permissões", "codusu", codUsu, "error", err)
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