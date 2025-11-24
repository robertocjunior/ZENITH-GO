package handler

import (
	"encoding/json"
	"net/http"
	"zenith-go/internal/auth"
	"zenith-go/internal/config"
	"zenith-go/internal/sankhya"
)

type AuthHandler struct {
	Client *sankhya.Client
	Config *config.Config
}

type loginInput struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DeviceToken string `json:"deviceToken"`
}

type logoutInput struct {
	SessionToken string `json:"sessionToken"`
}

type loginOutput struct {
	Username     string `json:"username"`
	CodUsu       int    `json:"codusu"` // Novo campo
	SessionToken string `json:"sessionToken"`
	SnkSessionID string `json:"snkjsessionid"`
	DeviceToken  string `json:"deviceToken"`
}

// HandleLogin processa a autenticação
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

	// 1. Verifica existência e permissão do usuário (Passo NOVO)
	// Se falhar aqui, já retorna erro 403 (Forbidden) ou 404
	codUsuFloat, err := h.Client.VerifyUserAccess(input.Username)
	if err != nil {
		// Retornamos 403 Forbidden para erros de regra de negócio/permissão
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// 2. Lógica do Device Token
	finalDeviceToken := input.DeviceToken
	if finalDeviceToken == "" {
		finalDeviceToken = auth.NewDeviceToken()
	}

	// 3. Login no Sankhya (Valida senha)
	snkJSession, err := h.Client.LoginUser(input.Username, input.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 4. Gera Token JWT da Zenith
	jwtToken, err := auth.GenerateToken(input.Username, h.Config.JwtSecret)
	if err != nil {
		http.Error(w, "Erro ao gerar sessão", http.StatusInternalServerError)
		return
	}

	// 5. Resposta com CODUSU
	response := loginOutput{
		Username:     input.Username,
		CodUsu:       int(codUsuFloat), // Converte float64 do JSON para int
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
	// (Lógica de logout permanece a mesma...)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}