package main

import (
	"log"
	"zenith-go/internal/config"
	"zenith-go/internal/sankhya"
)

func main() {
	// 1. Carregar configurações
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Erro ao carregar configurações: %v", err)
	}

	// 2. Inicializar o cliente do ERP
	sankhyaClient := sankhya.NewClient(cfg)

	// 3. Realizar o login inicial (Fail-fast: se falhar aqui, o app nem sobe)
	log.Println("Iniciando autenticação com o ERP...")
	if err := sankhyaClient.Authenticate(); err != nil {
		log.Fatalf("Erro crítico ao logar no ERP: %v", err)
	}

	// Exemplo: Simulando uso do token
	token, _ := sankhyaClient.GetToken()
	log.Printf("Servidor iniciado. Token atual em memória (início): %s...", token[:15])

	// Aqui você iniciaria seu servidor HTTP (ex: Gin, Chi ou http padrão)
	// http.ListenAndServe(":8080", r)
	select {} // Mantém o programa rodando para teste
}