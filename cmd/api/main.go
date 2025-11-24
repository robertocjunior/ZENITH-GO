package main

import (
	"log"
	"net/http"
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

	authHandler := &handler.AuthHandler{
		Client: sankhyaClient,
		Config: cfg,
	}

	mux := http.NewServeMux()
	
	// Rotas
	mux.HandleFunc("/apiv1/login", authHandler.HandleLogin)
	mux.HandleFunc("/apiv1/logout", authHandler.HandleLogout)
	mux.HandleFunc("/apiv1/permissions", authHandler.HandleGetPermissions) // Nova rota

	log.Println("Servidor rodando na porta :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Erro no servidor: %v", err)
	}
}