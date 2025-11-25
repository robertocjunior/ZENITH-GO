package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"
	"zenith-go/internal/auth"
	"zenith-go/internal/config"
	"zenith-go/internal/handler"
	"zenith-go/internal/logger" // Importe o pacote logger criado
	"zenith-go/internal/sankhya"
)

// responseWriter wrapper para capturar o status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Middleware de Segurança e CORS
func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Snkjsessionid")
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

// Middleware de Logging (NOVO)
// Registra cada requisição: Método, URL, Status, Duração e IP
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrapper para capturar o status code
		wrappedWriter := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrappedWriter, r)

		duration := time.Since(start)

		// Nível do log baseado no status
		logLevel := slog.LevelInfo
		if wrappedWriter.status >= 500 {
			logLevel = slog.LevelError
		} else if wrappedWriter.status >= 400 {
			logLevel = slog.LevelWarn
		}

		slog.Log(r.Context(), logLevel, "HTTP Request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", wrappedWriter.status),
			slog.String("duration", duration.String()),
			slog.String("ip", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
		)
	})
}

func main() {
	// 1. Inicializa o Sistema de Logs
	logger.Init()

	// Carrega as configurações
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Erro fatal ao carregar configurações", "error", err)
		panic(err) // Encerra se não tiver config
	}

	slog.Info("Configurações carregadas", "api_url", cfg.ApiUrl)

	// Inicializa o cliente Sankhya
	sankhyaClient := sankhya.NewClient(cfg)

	slog.Info("Autenticando sistema no ERP...")
	ctxBg := context.Background()
	if err := sankhyaClient.Authenticate(ctxBg); err != nil {
		slog.Error("Falha crítica no login do sistema", "error", err)
		panic(err)
	}
	slog.Info("Autenticação do sistema realizada com sucesso!")

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

	transactionHandler := &handler.TransactionHandler{
		Client:    sankhyaClient,
		Session:   sessionManager,
		JwtSecret: cfg.JwtSecret,
	}

	mux := http.NewServeMux()

	// Rotas
	mux.HandleFunc("/apiv1/login", authHandler.HandleLogin)
	mux.HandleFunc("/apiv1/logout", authHandler.HandleLogout)
	mux.HandleFunc("/apiv1/permissions", authHandler.HandleGetPermissions)
	mux.HandleFunc("/apiv1/search-items", productHandler.HandleSearchItems)
	mux.HandleFunc("/apiv1/get-item-details", productHandler.HandleGetItemDetails)
	mux.HandleFunc("/apiv1/get-picking-locations", productHandler.HandleGetPickingLocations)
	mux.HandleFunc("/apiv1/get-history", productHandler.HandleGetHistory)
	mux.HandleFunc("/apiv1/execute-transaction", transactionHandler.HandleExecuteTransaction)

	// Encadeamento de Middlewares: Logging -> Security -> Mux
	finalHandler := loggingMiddleware(securityMiddleware(mux))

	slog.Info("Servidor rodando", "port", "8080")
	if err := http.ListenAndServe(":8080", finalHandler); err != nil {
		slog.Error("Erro fatal no servidor", "error", err)
	}
}