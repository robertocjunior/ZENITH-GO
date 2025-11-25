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

type ProductHandler struct {
	Client  *sankhya.Client
	Config  *config.Config
	Session *auth.SessionManager
}

// (Structs de Input mantidas...)
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
	CodUsu int    `json:"codUsu"`
}

func getTokenFromHeaderProduct(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

func (h *ProductHandler) HandleSearchItems(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	token := getTokenFromHeaderProduct(r)
	if token == "" {
		http.Error(w, "Token de sessão não fornecido", http.StatusUnauthorized)
		return
	}

	codUsu, err := auth.ValidateToken(token, h.Config.JwtSecret) // Recupera codUsu para log
	if err != nil {
		http.Error(w, "Token inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.Session.ValidateAndUpdate(token); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var input searchItemsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	slog.Info("Busca de produtos", "user", codUsu, "codArm", input.CodArm, "filtro", input.Filtro)

	rows, err := h.Client.SearchItems(ctx, input.CodArm, input.Filtro)
	if err != nil {
		slog.Error("Erro na busca de produtos", "error", err)
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "A busca demorou muito e foi cancelada (Timeout)", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "Erro na busca: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Debug("Busca retornou resultados", "count", len(rows))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rows)
}

func (h *ProductHandler) HandleGetItemDetails(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	token := getTokenFromHeaderProduct(r)
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

	var input getItemDetailsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if input.CodArm <= 0 || input.Sequencia == "" {
		http.Error(w, "Parâmetros inválidos", http.StatusBadRequest)
		return
	}

	slog.Debug("Buscando detalhes do item", "user", codUsu, "codArm", input.CodArm, "seq", input.Sequencia)

	item, err := h.Client.GetItemDetails(ctx, input.CodArm, input.Sequencia)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "Tempo limite excedido", http.StatusGatewayTimeout)
			return
		}
		if errors.Is(err, sankhya.ErrItemNotFound) {
			slog.Info("Item não encontrado", "codArm", input.CodArm, "seq", input.Sequencia)
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			slog.Error("Erro ao buscar detalhes", "error", err)
			http.Error(w, "Erro na busca: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func (h *ProductHandler) HandleGetPickingLocations(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	token := getTokenFromHeaderProduct(r)
	if token == "" {
		http.Error(w, "Token de sessão não fornecido", http.StatusUnauthorized)
		return
	}

	if _, err := auth.ValidateToken(token, h.Config.JwtSecret); err != nil {
		http.Error(w, "Token inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.Session.ValidateAndUpdate(token); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var input getPickingLocationsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	slog.Debug("Buscando locais de picking", "codArm", input.CodArm, "codProd", input.CodProd)

	locations, err := h.Client.GetPickingLocations(ctx, input.CodArm, input.CodProd, input.Sequencia)
	if err != nil {
		slog.Error("Erro ao buscar locais de picking", "error", err)
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "Tempo limite excedido", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "Erro na busca: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(locations)
}

func (h *ProductHandler) HandleGetHistory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	token := getTokenFromHeaderProduct(r)
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

	var input getHistoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	slog.Info("Consulta de Histórico", "user", codUsu, "dtIni", input.DtIni, "dtFim", input.DtFim)

	history, err := h.Client.GetHistory(ctx, input.DtIni, input.DtFim, input.CodUsu)
	if err != nil {
		slog.Error("Erro ao consultar histórico", "error", err)
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "Tempo limite excedido", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "Erro na busca: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}