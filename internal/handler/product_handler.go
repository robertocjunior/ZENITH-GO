package handler

import (
	"encoding/json"
	"net/http"
	"zenith-go/internal/auth"
	"zenith-go/internal/config"
	"zenith-go/internal/sankhya"
)

type ProductHandler struct {
	Client  *sankhya.Client
	Config  *config.Config
	Session *auth.SessionManager
}

type searchItemsInput struct {
	SessionToken string `json:"sessionToken"`
	CodArm       int    `json:"codArm"`
	Filtro       string `json:"filtro"`
}

// HandleSearchItems realiza a busca de produtos no armazém
func (h *ProductHandler) HandleSearchItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var input searchItemsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// 1. Validação de Segurança (Token e Sessão)
	if _, err := auth.ValidateToken(input.SessionToken, h.Config.JwtSecret); err != nil {
		http.Error(w, "Token inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.Session.ValidateAndUpdate(input.SessionToken); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 2. Executa a busca no Sankhya
	rows, err := h.Client.SearchItems(input.CodArm, input.Filtro)
	if err != nil {
		http.Error(w, "Erro na busca: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Retorna os dados
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rows)
}