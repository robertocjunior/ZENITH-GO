package main

import (
	"log"
	"net/http"
	"zenith-go/internal/auth"
	"zenith-go/internal/config"
	"zenith-go/internal/handler"
	"zenith-go/internal/sankhya"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	sankhyaClient := sankhya.NewClient(cfg)

	log.Println("Autenticando sistema no ERP...")
	if err := sankhyaClient.Authenticate(); err != nil {
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

	// Rotas de Autenticação
	mux.HandleFunc("/apiv1/login", authHandler.HandleLogin)
	mux.HandleFunc("/apiv1/logout", authHandler.HandleLogout)
	mux.HandleFunc("/apiv1/permissions", authHandler.HandleGetPermissions)

	// Rotas de Produto
	mux.HandleFunc("/apiv1/search-items", productHandler.HandleSearchItems)

	log.Println("Servidor rodando na porta :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Erro no servidor: %v", err)
	}
}