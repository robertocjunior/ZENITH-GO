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
// Intercepta todas as requisições para aplicar headers de segurança e permitir acesso do frontend
func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Configuração de CORS (Crucial para o Frontend acessar a API)
		// Em produção, recomenda-se trocar "*" pelo domínio específico, ex: "https://meuapp.com.br"
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// 2. Headers de Segurança (Boas Práticas OWASP)
		w.Header().Set("X-Content-Type-Options", "nosniff") // Evita que o browser "adivinhe" o tipo de arquivo
		w.Header().Set("X-Frame-Options", "DENY")           // Evita ataques de Clickjacking (iframe)
		w.Header().Set("X-XSS-Protection", "1; mode=block") // Proteção básica contra XSS

		// Se for requisição OPTIONS (Preflight do browser), retorna OK imediatamente
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Carrega as configurações do .env
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	// Inicializa o cliente Sankhya
	sankhyaClient := sankhya.NewClient(cfg)

	log.Println("Autenticando sistema no ERP...")
	
	// CRIA UM CONTEXTO DE FUNDO PARA O LOGIN INICIAL
	// Necessário pois o método Authenticate agora exige um contexto
	ctxBg := context.Background()
	if err := sankhyaClient.Authenticate(ctxBg); err != nil {
		log.Fatalf("Falha crítica no login do sistema: %v", err)
	}

	// Inicializa o Gerenciador de Sessão com 50 minutos de timeout
	sessionManager := auth.NewSessionManager(50)

	// Inicializa Handlers
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

	mux := http.NewServeMux()

	// --- Rotas de Autenticação ---
	mux.HandleFunc("/apiv1/login", authHandler.HandleLogin)
	mux.HandleFunc("/apiv1/logout", authHandler.HandleLogout)
	mux.HandleFunc("/apiv1/permissions", authHandler.HandleGetPermissions)

	// --- Rotas de Produto ---
	mux.HandleFunc("/apiv1/search-items", productHandler.HandleSearchItems)
	mux.HandleFunc("/apiv1/get-item-details", productHandler.HandleGetItemDetails) // Nova rota adicionada

	log.Println("Servidor rodando na porta :8080")
	
	// Envolve o mux (roteador) com o middleware de segurança antes de iniciar o servidor
	if err := http.ListenAndServe(":8080", securityMiddleware(mux)); err != nil {
		log.Fatalf("Erro no servidor: %v", err)
	}
}