package handler

import (
	"encoding/json"
	"net/http"
	"time"
	"context"
	"zenith-go/internal/auth"
	"zenith-go/internal/sankhya"
)

// HandleGetRomaneioDetalhes processa a busca detalhada de um romaneio específico
func (h *RomaneioHandler) HandleGetRomaneioDetalhes(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second) // Query complexa, timeout maior
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// Validação de Segurança (Padrão Zenith)
	token := getTokenFromHeader(r)
	if token == "" {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token ausente", nil)
		return
	}

	if _, _, err := auth.ValidateToken(token, h.Config.JwtSecret); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token inválido", err)
		return
	}

	if err := h.Session.ValidateAndUpdate(token); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Sessão expirada", err)
		return
	}

	// Parsing do Body
	var input sankhya.RomaneioDetalheInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	if input.NumeroFechamento == 0 {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "Número do fechamento é obrigatório", nil)
		return
	}

	// Chamada ao Service
	detalhes, err := h.Client.GetRomaneioDetalhes(ctx, input.NumeroFechamento)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro ao buscar detalhes do romaneio", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detalhes)
}