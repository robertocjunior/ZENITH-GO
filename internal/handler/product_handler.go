package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
	SessionToken string `json:"sessionToken"`
	CodArm       int    `json:"codArm"`
	Filtro       string `json:"filtro"`
}

type getItemDetailsInput struct {
	SessionToken string `json:"sessionToken"`
	CodArm       int    `json:"codArm"`
	Sequencia    string `json:"sequencia"`
}

type getPickingLocationsInput struct {
	SessionToken string `json:"sessionToken"`
	CodArm       int    `json:"codarm"`
	CodProd      int    `json:"codprod"`
	Sequencia    int    `json:"sequencia"`
}

func (h *ProductHandler) HandleSearchItems(w http.ResponseWriter, r *http.Request) {
	// TIMEOUT: 30 segundos para buscar itens
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var input searchItemsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// 1. Validação de Segurança
	if _, err := auth.ValidateToken(input.SessionToken, h.Config.JwtSecret); err != nil {
		http.Error(w, "Token inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.Session.ValidateAndUpdate(input.SessionToken); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 2. Busca com Contexto
	rows, err := h.Client.SearchItems(ctx, input.CodArm, input.Filtro)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "A busca demorou muito e foi cancelada (Timeout)", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "Erro na busca: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Retorna os dados
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

	// 1. Segurança
	if _, err := auth.ValidateToken(input.SessionToken, h.Config.JwtSecret); err != nil {
		http.Error(w, "Token inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.Session.ValidateAndUpdate(input.SessionToken); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 2. Busca com Contexto
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

	// 3. Retorna os dados
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

// HandleGetPickingLocations busca locais de picking alternativos
func (h *ProductHandler) HandleGetPickingLocations(w http.ResponseWriter, r *http.Request) {
	// TIMEOUT: 30 segundos
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

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

	// 1. Segurança
	if _, err := auth.ValidateToken(input.SessionToken, h.Config.JwtSecret); err != nil {
		http.Error(w, "Token inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.Session.ValidateAndUpdate(input.SessionToken); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 2. Busca com Contexto
	locations, err := h.Client.GetPickingLocations(ctx, input.CodArm, input.CodProd, input.Sequencia)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "Tempo limite excedido", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "Erro na busca: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Retorna os dados
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(locations)
}