package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"zenith-go/internal/auth"
	"zenith-go/internal/config"
	"zenith-go/internal/handler"
	"zenith-go/internal/logger"
	"zenith-go/internal/sankhya"
)

// responseWriter wrapper para capturar o status code no middleware de log
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

// Middleware de Logging (Slog + Tint)
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrappedWriter := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		
		next.ServeHTTP(wrappedWriter, r)
		
		// --- ALTERAÇÃO AQUI ---
		// Se for health check, não gera log para não poluir
		if r.URL.Path == "/apiv1/health" {
			return
		}
		// ---------------------

		duration := time.Since(start)

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
	// 1. Carrega Configurações
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Erro fatal ao carregar configurações: %v\n", err)
		panic(err)
	}

	// 2. Inicializa Logger
	logger.Init(cfg)
	slog.Info("Configurações carregadas", "api_url", cfg.ApiUrl)

	// 3. Inicializa Cliente Sankhya e Autentica o Sistema
	sankhyaClient := sankhya.NewClient(cfg)

	slog.Info("Autenticando sistema no ERP...")
	ctxBg := context.Background()
	if err := sankhyaClient.Authenticate(ctxBg); err != nil {
		slog.Error("Falha crítica no login do sistema", "error", err)
		// Panic aqui pois o sistema não funciona sem conexão com o ERP
		panic(err)
	}
	slog.Info("Autenticação do sistema realizada com sucesso!")

	// 4. Inicializa Gerenciador de Sessão (Redis)
	// Timeout de 50 minutos para as sessões
	slog.Info("Conectando ao Redis...", "addr", cfg.RedisAddr)
	sessionManager, err := auth.NewSessionManager(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, 50)
	if err != nil {
		slog.Error("Falha fatal ao conectar no Redis", "error", err)
		panic(err)
	}
	slog.Info("Conexão com Redis estabelecida com sucesso")

	// 5. Inicializa Handlers
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

	healthHandler := &handler.HealthHandler{
        Session: sessionManager,
        Config:  cfg, // Passa a config carregada
    }

	// 6. Configura Rotas
	mux := http.NewServeMux()

	// Autenticação
	mux.HandleFunc("/apiv1/login", authHandler.HandleLogin)
	mux.HandleFunc("/apiv1/logout", authHandler.HandleLogout)
	mux.HandleFunc("/apiv1/permissions", authHandler.HandleGetPermissions)

	// Produtos e Estoque
	mux.HandleFunc("/apiv1/search-items", productHandler.HandleSearchItems)
	mux.HandleFunc("/apiv1/get-item-details", productHandler.HandleGetItemDetails)
	mux.HandleFunc("/apiv1/get-picking-locations", productHandler.HandleGetPickingLocations)
	mux.HandleFunc("/apiv1/get-history", productHandler.HandleGetHistory)

	// Transações
	mux.HandleFunc("/apiv1/execute-transaction", transactionHandler.HandleExecuteTransaction)

	// Monitoramento e Saúde (NOVO)
	mux.HandleFunc("/apiv1/health", healthHandler.HandleHealthCheck)

	// Aplica Middlewares
	finalHandler := loggingMiddleware(securityMiddleware(mux))

	// 7. Configuração do Servidor HTTP
	srv := &http.Server{
		Addr:    ":8080",
		Handler: finalHandler,
	}

	// 8. Graceful Shutdown
	// Canal para escutar sinais do SO (CTRL+C ou Docker Stop)
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Inicia o servidor em uma Goroutine separada
	go func() {
		slog.Info("Servidor rodando", "port", "8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Erro fatal no servidor", "error", err)
			os.Exit(1)
		}
	}()

	// Bloqueia até receber um sinal de parada
	sig := <-stopChan
	slog.Info("Sinal de encerramento recebido", "signal", sig)

	// Cria um contexto com timeout para o shutdown (10 segundos)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	slog.Info("Iniciando desligamento gracioso (Graceful Shutdown)...")
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Erro ao desligar servidor forçadamente", "error", err)
	}

	slog.Info("Servidor desligado com sucesso.")
}