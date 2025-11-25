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

type ProductHandler struct {
	Client  *sankhya.Client
	Config  *config.Config
	Session *auth.SessionManager
}

type searchItemsInput struct {
	CodArm int    `json:"codArm"`
	Filtro string `json:"filtro"`
}

type getItemDetailsInput struct {
	CodArm    int    `json:"codArm"`
	Sequencia string `json:"sequencia"`
}

type getPickingLocationsInput struct {
	CodArm    int `json:"codarm"`
	CodProd   int `json:"codprod"`
	Sequencia int `json:"sequencia"`
}

type getHistoryInput struct {
	DtIni  string `json:"dtIni"`
	DtFim  string `json:"dtFim"`
	CodUsu int    `json:"codUsu"` // Opcional
}

// Helper para extrair token do Header Authorization
func getTokenFromHeaderProduct(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

func (h *ProductHandler) HandleSearchItems(w http.ResponseWriter, r *http.Request) {
	// TIMEOUT: 30 segundos para buscar itens
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// 1. Extração do Token do Header
	token := getTokenFromHeaderProduct(r)
	if token == "" {
		http.Error(w, "Token de sessão não fornecido", http.StatusUnauthorized)
		return
	}

	// 2. Validação de Segurança (Token e Sessão)
	if _, err := auth.ValidateToken(token, h.Config.JwtSecret); err != nil {
		http.Error(w, "Token inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.Session.ValidateAndUpdate(token); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 3. Decode do Body
	var input searchItemsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// 4. Busca com Contexto
	rows, err := h.Client.SearchItems(ctx, input.CodArm, input.Filtro)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "A busca demorou muito e foi cancelada (Timeout)", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "Erro na busca: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. Retorna os dados
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rows)
}

func (h *ProductHandler) HandleGetItemDetails(w http.ResponseWriter, r *http.Request) {
	// TIMEOUT: 30 segundos
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// 1. Extração do Token do Header
	token := getTokenFromHeaderProduct(r)
	if token == "" {
		http.Error(w, "Token de sessão não fornecido", http.StatusUnauthorized)
		return
	}

	// 2. Segurança
	if _, err := auth.ValidateToken(token, h.Config.JwtSecret); err != nil {
		http.Error(w, "Token inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.Session.ValidateAndUpdate(token); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 3. Decode do Body
	var input getItemDetailsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Validação Input Básico
	if input.CodArm <= 0 {
		http.Error(w, "Código do armazém inválido", http.StatusBadRequest)
		return
	}
	if input.Sequencia == "" {
		http.Error(w, "Sequência inválida", http.StatusBadRequest)
		return
	}

	// 4. Busca com Contexto
	item, err := h.Client.GetItemDetails(ctx, input.CodArm, input.Sequencia)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "Tempo limite excedido ao buscar detalhes", http.StatusGatewayTimeout)
			return
		}
		if errors.Is(err, sankhya.ErrItemNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, "Erro na busca: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// 5. Retorna os dados
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func (h *ProductHandler) HandleGetPickingLocations(w http.ResponseWriter, r *http.Request) {
	// TIMEOUT: 30 segundos
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// 1. Extração do Token do Header
	token := getTokenFromHeaderProduct(r)
	if token == "" {
		http.Error(w, "Token de sessão não fornecido", http.StatusUnauthorized)
		return
	}

	// 2. Segurança
	if _, err := auth.ValidateToken(token, h.Config.JwtSecret); err != nil {
		http.Error(w, "Token inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.Session.ValidateAndUpdate(token); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 3. Decode do Body
	var input getPickingLocationsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Validação Básica
	if input.CodArm <= 0 || input.CodProd <= 0 {
		http.Error(w, "Parâmetros inválidos", http.StatusBadRequest)
		return
	}

	// 4. Busca com Contexto
	locations, err := h.Client.GetPickingLocations(ctx, input.CodArm, input.CodProd, input.Sequencia)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "Tempo limite excedido", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "Erro na busca: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. Retorna os dados
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(locations)
}

// HandleGetHistory busca o histórico
func (h *ProductHandler) HandleGetHistory(w http.ResponseWriter, r *http.Request) {
	// TIMEOUT: 60 segundos (Relatórios podem demorar mais)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// 1. Token Header
	token := getTokenFromHeaderProduct(r)
	if token == "" {
		http.Error(w, "Token de sessão não fornecido", http.StatusUnauthorized)
		return
	}

	// 2. Segurança
	if _, err := auth.ValidateToken(token, h.Config.JwtSecret); err != nil {
		http.Error(w, "Token inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.Session.ValidateAndUpdate(token); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 3. Decode Body
	var input getHistoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Validação
	if input.DtIni == "" || input.DtFim == "" {
		http.Error(w, "Datas inicial e final são obrigatórias", http.StatusBadRequest)
		return
	}

	// 4. Busca
	history, err := h.Client.GetHistory(ctx, input.DtIni, input.DtFim, input.CodUsu)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "Tempo limite excedido", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "Erro na busca: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. Retorno
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}