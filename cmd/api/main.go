package main

import (
	"log"
	"net/http"
	"zenith-go/internal/config"
	"zenith-go/internal/handler"
	"zenith-go/internal/sankhya"
)

func main() {
	// 1. Config e Dependências
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	sankhyaClient := sankhya.NewClient(cfg)

	// 2. Autenticação Inicial do Sistema (Fail-fast)
	log.Println("Autenticando sistema no ERP...")
	if err := sankhyaClient.Authenticate(); err != nil {
		log.Fatalf("Falha crítica no login do sistema: %v", err)
	}

	// 3. Handlers
	authHandler := &handler.AuthHandler{
		Client: sankhyaClient,
		Config: cfg,
	}

	// 4. Roteamento
	mux := http.NewServeMux()
	
	// Rotas Públicas
	mux.HandleFunc("/apiv1/login", authHandler.HandleLogin)
	mux.HandleFunc("/apiv1/logout", authHandler.HandleLogout)

	// 5. Start Server
	log.Println("Servidor rodando na porta :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Erro no servidor: %v", err)
	}
}