package main

import (
	"context"
	"log"
	"net/http"
	"zenith-go/internal/auth"
	"zenith-go/internal/config"
	"zenith-go/internal/handler"
	"zenith-go/internal/sankhya"
)

// Middleware de Segurança e CORS
func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Snkjsessionid") // Adicionado Snkjsessionid

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	sankhyaClient := sankhya.NewClient(cfg)

	log.Println("Autenticando sistema no ERP...")
	ctxBg := context.Background()
	if err := sankhyaClient.Authenticate(ctxBg); err != nil {
		log.Fatalf("Falha crítica no login do sistema: %v", err)
	}

	sessionManager := auth.NewSessionManager(50)

	authHandler := &handler.AuthHandler{
		Client:  sankhyaClient,
		Config:  cfg,
		Session: sessionManager,
	}

	productHandler := &handler.ProductHandler{
		Client:  sankhyaClient,
		Config:  cfg,
		Session: sessionManager,
	}

	// Inicializa TransactionHandler
	transactionHandler := &handler.TransactionHandler{
		Client:    sankhyaClient,
		Session:   sessionManager,
		JwtSecret: cfg.JwtSecret,
	}

	mux := http.NewServeMux()

	// --- Rotas de Autenticação ---
	mux.HandleFunc("/apiv1/login", authHandler.HandleLogin)
	mux.HandleFunc("/apiv1/logout", authHandler.HandleLogout)
	mux.HandleFunc("/apiv1/permissions", authHandler.HandleGetPermissions)

	// --- Rotas de Produto ---
	mux.HandleFunc("/apiv1/search-items", productHandler.HandleSearchItems)
	mux.HandleFunc("/apiv1/get-item-details", productHandler.HandleGetItemDetails)
	mux.HandleFunc("/apiv1/get-picking-locations", productHandler.HandleGetPickingLocations)
	mux.HandleFunc("/apiv1/get-history", productHandler.HandleGetHistory)

	// --- Nova Rota de Transação ---
	mux.HandleFunc("/apiv1/execute-transaction", transactionHandler.HandleExecuteTransaction)

	log.Println("Servidor rodando na porta :8080")
	if err := http.ListenAndServe(":8080", securityMiddleware(mux)); err != nil {
		log.Fatalf("Erro no servidor: %v", err)
	}
}