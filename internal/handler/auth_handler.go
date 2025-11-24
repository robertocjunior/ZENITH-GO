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
	Client *sankhya.Client
	Config *config.Config
}

// --- Structs de Entrada ---
type loginInput struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DeviceToken string `json:"deviceToken"`
}

type logoutInput struct {
	SessionToken string `json:"sessionToken"`
}

// --- Structs de Saída ---
type loginOutput struct {
	Username     string `json:"username"`
	CodUsu       int    `json:"codusu"`
	SessionToken string `json:"sessionToken"`
	SnkSessionID string `json:"snkjsessionid"`
	DeviceToken  string `json:"deviceToken"`
}

// HandleLogin processa o login completo (Verificação -> Device Token -> Login ERP -> JWT)
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// 1. Parse do JSON
	var input loginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// 2. Verificação de Acesso (User existe? Tem permissão?)
	codUsuFloat, err := h.Client.VerifyUserAccess(input.Username)
	if err != nil {
		// Diferenciação de Erros para o Frontend
		if errors.Is(err, sankhya.ErrUserNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound) // 404
		} else if errors.Is(err, sankhya.ErrUserNotAuthorized) {
			http.Error(w, err.Error(), http.StatusForbidden) // 403
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError) // 500
		}
		return
	}

	// 3. Gerenciamento de Device Token (Anti-Colisão UUID)
	finalDeviceToken := input.DeviceToken
	if finalDeviceToken == "" {
		finalDeviceToken = auth.NewDeviceToken()
	}

	// 4. Login Efetivo no Sankhya (Validação de Senha)
	snkJSession, err := h.Client.LoginUser(input.Username, input.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 5. Geração do JWT da Aplicação (50 min)
	jwtToken, err := auth.GenerateToken(input.Username, h.Config.JwtSecret)
	if err != nil {
		http.Error(w, "Erro ao gerar sessão interna", http.StatusInternalServerError)
		return
	}

	// 6. Montagem da Resposta
	response := loginOutput{
		Username:     input.Username,
		CodUsu:       int(codUsuFloat),
		SessionToken: jwtToken,
		SnkSessionID: snkJSession,
		DeviceToken:  finalDeviceToken,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleLogout encerra a sessão (simbólico em JWT Stateless)
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var input logoutInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Aqui você pode adicionar lógica de blacklist para o token se desejar

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}