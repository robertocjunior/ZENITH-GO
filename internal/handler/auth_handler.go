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

// HandleLogin processa todo o fluxo de autenticação
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

	// 1. Gerenciamento de Device Token (UUID Novo ou Existente)
	finalDeviceToken := input.DeviceToken
	if finalDeviceToken == "" {
		finalDeviceToken = auth.NewDeviceToken()
	}

	// 2. Verificar Acesso do Usuário (Existe? Tem Permissão?)
	// Retorna o CODUSU necessário para os próximos passos
	codUsuFloat, err := h.Client.VerifyUserAccess(input.Username)
	if err != nil {
		if errors.Is(err, sankhya.ErrUserNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound) // 404
		} else if errors.Is(err, sankhya.ErrUserNotAuthorized) {
			http.Error(w, err.Error(), http.StatusForbidden) // 403
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	codUsu := int(codUsuFloat)

	// 3. Verificar Liberação do Dispositivo
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

	// 4. Login Efetivo no Sankhya (Validação de Senha)
	snkJSession, err := h.Client.LoginUser(input.Username, input.Password)
	if err != nil {
		http.Error(w, "Credenciais inválidas", http.StatusUnauthorized)
		return
	}

	// 5. Gera o token JWT da API incluindo o CODUSU no payload
	jwtToken, err := auth.GenerateToken(input.Username, codUsu, h.Config.JwtSecret)
	if err != nil {
		http.Error(w, "Erro ao gerar sessão", http.StatusInternalServerError)
		return
	}

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

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// HandleGetPermissions busca permissões usando o CODUSU extraído do Token
// Rota: POST /apiv1/permissions
// Body: { "sessionToken": "..." }
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

	// 1. Valida o token e extrai o CODUSU (Segurança)
	codUsu, err := auth.ValidateToken(input.SessionToken, h.Config.JwtSecret)
	if err != nil {
		http.Error(w, "Sessão inválida ou expirada: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// 2. Busca permissões no ERP usando o ID confiável do token
	permissions, err := h.Client.GetUserPermissions(codUsu)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(permissions)
}