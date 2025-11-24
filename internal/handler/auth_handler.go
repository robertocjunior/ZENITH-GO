package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"zenith-go/internal/auth"
	"zenith-go/internal/config"
	"zenith-go/internal/sankhya"
)

type AuthHandler struct {
	Client  *sankhya.Client
	Config  *config.Config
	Session *auth.SessionManager // Novo campo
}

type loginInput struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DeviceToken string `json:"deviceToken"`
}

type sessionInput struct {
	SessionToken string `json:"sessionToken"`
}

type loginOutput struct {
	Username     string `json:"username"`
	CodUsu       int    `json:"codusu"`
	SessionToken string `json:"sessionToken"`
	SnkSessionID string `json:"snkjsessionid"`
	DeviceToken  string `json:"deviceToken"`
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
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

	codUsuFloat, err := h.Client.VerifyUserAccess(input.Username)
	if err != nil {
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

	if err := h.Client.VerifyDevice(codUsu, finalDeviceToken); err != nil {
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

	snkJSession, err := h.Client.LoginUser(input.Username, input.Password)
	if err != nil {
		http.Error(w, "Credenciais inválidas", http.StatusUnauthorized)
		return
	}

	// Gera Token
	jwtToken, err := auth.GenerateToken(input.Username, codUsu, h.Config.JwtSecret)
	if err != nil {
		http.Error(w, "Erro ao gerar sessão", http.StatusInternalServerError)
		return
	}

	// --- REGISTRA A SESSÃO NA MEMÓRIA ---
	h.Session.Register(jwtToken)
	// ------------------------------------

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

	var input sessionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// --- REVOGA A SESSÃO DA MEMÓRIA ---
	h.Session.Revoke(input.SessionToken)
	// ----------------------------------

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *AuthHandler) HandleGetPermissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido (use POST com sessionToken)", http.StatusMethodNotAllowed)
		return
	}

	var input sessionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// 1. Validação JWT (Assinatura)
	codUsu, err := auth.ValidateToken(input.SessionToken, h.Config.JwtSecret)
	if err != nil {
		http.Error(w, "Token inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// 2. Validação de Sessão (Tempo e Reinício)
	// Verifica se está na memória e se não passou de 50min
	if err := h.Session.ValidateAndUpdate(input.SessionToken); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	permissions, err := h.Client.GetUserPermissions(codUsu)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(permissions)
}