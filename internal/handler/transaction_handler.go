package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"zenith-go/internal/auth"
	"zenith-go/internal/sankhya"
)

// TransactionHandler lida com transações de estoque
type TransactionHandler struct {
	Client  *sankhya.Client
	Session *auth.SessionManager
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

func (h *TransactionHandler) HandleExecuteTransaction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second) // Transações podem ser longas
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// 1. Headers Obrigatórios
	bearerToken := getTokenFromHeaderProduct(r) // Reutilizando helper do product_handler ou duplicando logica
	if bearerToken == "" {
		bearerHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(bearerHeader, "Bearer ") {
			bearerToken = strings.TrimPrefix(bearerHeader, "Bearer ")
		}
	}
	
	snkSessionId := getHeader(r, "Snkjsessionid")

	if bearerToken == "" || snkSessionId == "" {
		http.Error(w, "Headers Authorization e Snkjsessionid são obrigatórios", http.StatusUnauthorized)
		return
	}

	// 2. Validação JWT (Nosso Token)
	codUsu, err := auth.ValidateToken(bearerToken, h.JwtSecret)
	if err != nil {
		http.Error(w, "Token API inválido: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Validação Sessão (Memória)
	if err := h.Session.ValidateAndUpdate(bearerToken); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 3. Body
	var req transactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// 4. Execução
	input := sankhya.TransactionInput{
		Type:    req.Type,
		Payload: req.Payload,
		CodUsu:  codUsu,
	}

	msg, err := h.Client.ExecuteTransaction(ctx, input, snkSessionId)
	if err != nil {
		// Se for erro de permissão, 403, senão 500
		if strings.Contains(err.Error(), "permissão") {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, "Erro na transação: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": msg})
}