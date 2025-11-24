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

type logoutInput struct {
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

	// 1. Parse Input
	var input loginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// 2. Definir Device Token (UUID Novo ou Existente)
	finalDeviceToken := input.DeviceToken
	if finalDeviceToken == "" {
		finalDeviceToken = auth.NewDeviceToken()
	}

	// 3. Verificar Acesso do Usuário (Existe? Tem Permissão?)
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

	// 4. Verificar Liberação do Dispositivo (Novo passo)
	// Se não existir, ele cria e retorna erro. Se existir inativo, retorna erro.
	if err := h.Client.VerifyDevice(codUsu, finalDeviceToken); err != nil {
		if errors.Is(err, sankhya.ErrDevicePendingApproval) {
			// Retornamos 403 (Forbidden) com a mensagem explicativa para o App exibir
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error":       err.Error(),
				"deviceToken": finalDeviceToken, // Importante retornar o Token para o App salvar
			})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. Login Efetivo no Sankhya (Validação de Senha)
	snkJSession, err := h.Client.LoginUser(input.Username, input.Password)
	if err != nil {
		http.Error(w, "Credenciais inválidas", http.StatusUnauthorized)
		return
	}

	// 6. Gerar JWT
	jwtToken, err := auth.GenerateToken(input.Username, h.Config.JwtSecret)
	if err != nil {
		http.Error(w, "Erro ao gerar sessão", http.StatusInternalServerError)
		return
	}

	// 7. Resposta de Sucesso
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

	var input logoutInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}