package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
	"strings"
	"zenith-go/internal/auth"
	"zenith-go/internal/config"
	"zenith-go/internal/notification"
	"zenith-go/internal/sankhya"
)

type RomaneioHandler struct {
	Client   *sankhya.Client
	Config   *config.Config
	Session  *auth.SessionManager
	Notifier *notification.EmailService
}

func (h *RomaneioHandler) HandleGetRomaneios(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// Reutiliza a lógica de extração de token do projeto
	token := getTokenFromHeader(r)
	if token == "" {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token ausente", nil)
		return
	}

	// Validação do JWT
	_, _, err := auth.ValidateToken(token, h.Config.JwtSecret)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token inválido", err)
		return
	}

	// Atualiza sessão no Redis (Sliding Expiration)
	if err := h.Session.ValidateAndUpdate(token); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Sessão expirada", err)
		return
	}

	var input sankhya.RomaneioInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	if input.Data == "" {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "O campo 'data' é obrigatório", nil)
		return
	}

	data, err := h.Client.GetRomaneios(ctx, input.Data)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro ao buscar romaneios", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func getTokenFromHeaderRomaneio(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

func getHeaderRomaneio(r *http.Request, key string) string {
	return r.Header.Get(key)
}

func (h *RomaneioHandler) HandleIniciarConferencia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// 1. Extração dos Tokens (Igual ao TransactionHandler)
	bearerToken := getTokenFromHeaderRomaneio(r)
	snkSessionId := getHeaderRomaneio(r, "Snkjsessionid")

	if bearerToken == "" || snkSessionId == "" {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Tokens ausentes (Authorization ou Snkjsessionid)", nil)
		return
	}

	// 2. Validação do JWT
	if _, _, err := auth.ValidateToken(bearerToken, h.Config.JwtSecret); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token inválido", err)
		return
	}

	// 3. Validação da Sessão (Sliding Expiration)
	if err := h.Session.ValidateAndUpdate(bearerToken); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Sessão expirada", err)
		return
	}

	// 4. Parsing do Body
	var input sankhya.IniciarConferenciaInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	if input.NuUnico == 0 {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "O campo 'nu_unico' é obrigatório", nil)
		return
	}

	// 5. Execução (Passando o snkSessionId)
	ctx := r.Context()
	resp, err := h.Client.IniciarConferencia(ctx, input.NuUnico, snkSessionId)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro ao iniciar conferência", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *RomaneioHandler) HandleConferirItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// 1. Extração e Validação dos Tokens
	bearerToken := getTokenFromHeaderRomaneio(r)
	snkSessionId := getHeaderRomaneio(r, "Snkjsessionid")

	if bearerToken == "" || snkSessionId == "" {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Tokens ausentes (Authorization ou Snkjsessionid)", nil)
		return
	}

	if _, _, err := auth.ValidateToken(bearerToken, h.Config.JwtSecret); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token inválido", err)
		return
	}

	if err := h.Session.ValidateAndUpdate(bearerToken); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Sessão expirada", err)
		return
	}

	// 2. Parsing do Body
	var input sankhya.ConferirItemInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	// 3. Validação de campos obrigatórios
	// NUUNICO, NUMREG e QTDEMBARCADA são obrigatórios
	if input.NuUnico == 0 || input.NumReg == 0 {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "Os campos 'nu_unico' e 'num_reg' são obrigatórios", nil)
		return
	}

	if input.QtdEmbarcada <= 0 {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "O campo 'qtd_embarcada' deve ser maior que zero", nil)
		return
	}

	// 4. Execução (Passando o input completo)
	ctx := r.Context()
	resp, err := h.Client.ConferirItem(ctx, input, snkSessionId)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro ao conferir item", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *RomaneioHandler) HandleFinalizarConferencia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// 1. Extração e Validação dos Tokens
	bearerToken := getTokenFromHeaderRomaneio(r)
	snkSessionId := getHeaderRomaneio(r, "Snkjsessionid")

	if bearerToken == "" || snkSessionId == "" {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Tokens ausentes (Authorization ou Snkjsessionid)", nil)
		return
	}

	if _, _, err := auth.ValidateToken(bearerToken, h.Config.JwtSecret); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Token inválido", err)
		return
	}

	if err := h.Session.ValidateAndUpdate(bearerToken); err != nil {
		RespondError(w, r, h.Notifier, http.StatusUnauthorized, "Sessão expirada", err)
		return
	}

	// 2. Parsing do Body
	var input sankhya.FinalizarConferenciaInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	// 3. Validação de campos obrigatórios
	if input.NuUnico == 0 {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "O campo 'nu_unico' é obrigatório", nil)
		return
	}

	// 4. Execução
	ctx := r.Context()
	resp, err := h.Client.FinalizarConferencia(ctx, input, snkSessionId)
	if err != nil {
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Erro ao finalizar conferência", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}