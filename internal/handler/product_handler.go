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
	"zenith-go/internal/notification"
	"zenith-go/internal/sankhya"
)

type ProductHandler struct {
	Client   *sankhya.Client
	Config   *config.Config
	Session  *auth.SessionManager
	Notifier *notification.EmailService
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
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token ausente", nil)
		return
	}

	codUsu, err := auth.ValidateToken(token, h.Config.JwtSecret)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token inválido", err)
		return
	}

	if err := h.Session.ValidateAndUpdate(token); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Sessão expirada", err)
		return
	}

	var input searchItemsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	slog.Info("Busca de produtos", "user", codUsu, "codArm", input.CodArm, "filtro", input.Filtro)

	rows, err := h.Client.SearchItems(ctx, input.CodArm, input.Filtro)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro na busca de produtos", err)
		return
	}

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
	if _, err := auth.ValidateToken(token, h.Config.JwtSecret); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token inválido", err)
		return
	}
	if err := h.Session.ValidateAndUpdate(token); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Sessão expirada", err)
		return
	}

	var input getItemDetailsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	item, err := h.Client.GetItemDetails(ctx, input.CodArm, input.Sequencia)
	if err != nil {
		if errors.Is(err, sankhya.ErrItemNotFound) {
			RespondError(w, r, h.Notifier, http.StatusNotFound, err.Error(), nil)
		} else {
			RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro ao buscar detalhes", err)
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
	if _, err := auth.ValidateToken(token, h.Config.JwtSecret); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token inválido", err)
		return
	}
	if err := h.Session.ValidateAndUpdate(token); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Sessão expirada", err)
		return
	}

	var input getPickingLocationsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	locations, err := h.Client.GetPickingLocations(ctx, input.CodArm, input.CodProd, input.Sequencia)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro ao buscar picking", err)
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
	codUsu, err := auth.ValidateToken(token, h.Config.JwtSecret)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token inválido", err)
		return
	}
	if err := h.Session.ValidateAndUpdate(token); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Sessão expirada", err)
		return
	}

	var input getHistoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	// CORREÇÃO: Log adicionado para utilizar a variável codUsu
	slog.Info("Consulta de Histórico", "user", codUsu, "dtIni", input.DtIni, "dtFim", input.DtFim)

	history, err := h.Client.GetHistory(ctx, input.DtIni, input.DtFim, input.CodUsu)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro ao buscar histórico", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}