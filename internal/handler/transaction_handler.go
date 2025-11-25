package handler

import (
	"context"
	"encoding/json"
	"log/slog" // Importar slog
	"net/http"
	"strings"
	"time"
	"zenith-go/internal/auth"
	"zenith-go/internal/sankhya"
)

// TransactionHandler lida com transações de estoque
type TransactionHandler struct {
	Client    *sankhya.Client
	Session   *auth.SessionManager
	JwtSecret string
}

type transactionRequest struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

// Helper para pegar header específico
func getHeader(r *http.Request, key string) string {
	return r.Header.Get(key)
}

// Helper para extrair token (reutilizado)
func getTokenFromHeaderTrans(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

func (h *TransactionHandler) HandleExecuteTransaction(w http.ResponseWriter, r *http.Request) {
	// Contexto com timeout
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// 1. Headers e Autenticação
	bearerToken := getTokenFromHeaderTrans(r)
	snkSessionId := getHeader(r, "Snkjsessionid")

	if bearerToken == "" || snkSessionId == "" {
		slog.Warn("Tentativa de transação sem tokens", "ip", r.RemoteAddr)
		http.Error(w, "Headers Authorization e Snkjsessionid são obrigatórios", http.StatusUnauthorized)
		return
	}

	codUsu, err := auth.ValidateToken(bearerToken, h.JwtSecret)
	if err != nil {
		slog.Warn("Token JWT inválido na transação", "error", err, "ip", r.RemoteAddr)
		http.Error(w, "Token API inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.Session.ValidateAndUpdate(bearerToken); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 2. Body Decode
	var req transactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("JSON inválido na transação", "error", err)
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// LOG DE INÍCIO DE TRANSAÇÃO
	slog.Info("Iniciando transação",
		"user_cod", codUsu,
		"type", req.Type,
		"payload_summary", summarizePayload(req.Payload), // Helper opcional para não logar tudo se for gigante
	)

	// 3. Execução
	input := sankhya.TransactionInput{
		Type:    req.Type,
		Payload: req.Payload,
		CodUsu:  codUsu,
	}

	msg, err := h.Client.ExecuteTransaction(ctx, input, snkSessionId)
	if err != nil {
		// LOG DE ERRO DETALHADO
		slog.Error("Falha na execução da transação",
			"user_cod", codUsu,
			"type", req.Type,
			"error", err,
		)

		if strings.Contains(err.Error(), "permissão") {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, "Erro na transação: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// LOG DE SUCESSO
	slog.Info("Transação concluída com sucesso",
		"user_cod", codUsu,
		"type", req.Type,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": msg})
}

// Helper simples para logar resumo do payload (evita logar JSONs gigantescos)
func summarizePayload(payload map[string]any) string {
	if payload == nil {
		return "nil"
	}
	// Retorna apenas as chaves principais para debug
	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}