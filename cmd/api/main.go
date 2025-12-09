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
	"zenith-go/internal/notification"
	"zenith-go/internal/sankhya"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrappedWriter := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrappedWriter, r)
		
		if r.URL.Path == "/apiv1/health" { return }

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

// WORKER DE KEEP-ALIVE
func startKeepAliveWorker(session *auth.SessionManager, client *sankhya.Client) {
	// ALTERADO: Ticker mais rápido (5s) para atender o intervalo de 15s
	ticker := time.NewTicker(5 * time.Second) 
	go func() {
		for range ticker.C {
			tokens, err := session.GetTokensToPing()
			if err != nil {
				slog.Error("KeepAlive Worker: Erro ao buscar tokens", "error", err)
				continue
			}

			if len(tokens) > 0 {
				slog.Debug("KeepAlive Worker: Processando tokens", "count", len(tokens))
			}

			for _, token := range tokens {
				// Recupera o JSessionID
				jsid, err := session.GetSankhyaSession(token)
				if err != nil {
					// Se não achou sessão, remove da lista de ping
					session.Revoke(token)
					continue
				}

				// Faz o Ping no Sankhya
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err = client.KeepAlive(ctx, jsid)
				cancel()

				if err != nil {
					slog.Warn("KeepAlive Worker: Falha ao pingar Sankhya", "token_suffix", token[len(token)-5:], "error", err)
					continue 
				}

				// Sucesso: Reagenda para daqui a 15 segundos
				if err := session.UpdatePingTime(token); err != nil {
					slog.Error("KeepAlive Worker: Erro ao atualizar tempo", "error", err)
				}
			}
		}
	}()
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Erro fatal ao carregar configurações: %v\n", err)
		panic(err)
	}

	logger.Init(cfg)
	slog.Info("Configurações carregadas", "api_url", cfg.ApiUrl)

	emailService := notification.NewEmailService(cfg)
	sankhyaClient := sankhya.NewClient(cfg)

	slog.Info("Autenticando sistema no ERP...")
	ctxBg := context.Background()
	if err := sankhyaClient.Authenticate(ctxBg); err != nil {
		slog.Error("Falha crítica no login do sistema", "error", err)
		emailService.SendError(err, map[string]string{"Context": "Startup Authentication"})
		panic(err)
	}
	slog.Info("Autenticação do sistema realizada com sucesso!")

	slog.Info("Conectando ao Redis...", "addr", cfg.RedisAddr)
	sessionManager, err := auth.NewSessionManager(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, 50)
	if err != nil {
		slog.Error("Falha fatal ao conectar no Redis", "error", err)
		emailService.SendError(err, map[string]string{"Context": "Startup Redis Connection"})
		panic(err)
	}
	slog.Info("Conexão com Redis estabelecida com sucesso")

	// --- INICIA O WORKER DE KEEP-ALIVE ---
	slog.Info("Iniciando Worker de Keep-Alive (Sankhya)...")
	startKeepAliveWorker(sessionManager, sankhyaClient)
	// -------------------------------------

	authHandler := &handler.AuthHandler{
		Client:   sankhyaClient,
		Config:   cfg,
		Session:  sessionManager,
		Notifier: emailService,
	}

	productHandler := &handler.ProductHandler{
		Client:   sankhyaClient,
		Config:   cfg,
		Session:  sessionManager,
		Notifier: emailService,
	}

	transactionHandler := &handler.TransactionHandler{
		Client:    sankhyaClient,
		Session:   sessionManager,
		JwtSecret: cfg.JwtSecret,
		Notifier:  emailService,
	}

	healthHandler := &handler.HealthHandler{
		Session:  sessionManager,
		Config:   cfg,
		Notifier: emailService,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/apiv1/login", authHandler.HandleLogin)
	mux.HandleFunc("/apiv1/logout", authHandler.HandleLogout)
	mux.HandleFunc("/apiv1/permissions", authHandler.HandleGetPermissions)
	mux.HandleFunc("/apiv1/search-items", productHandler.HandleSearchItems)
	mux.HandleFunc("/apiv1/get-item-details", productHandler.HandleGetItemDetails)
	mux.HandleFunc("/apiv1/get-picking-locations", productHandler.HandleGetPickingLocations)
	mux.HandleFunc("/apiv1/get-history", productHandler.HandleGetHistory)
	mux.HandleFunc("/apiv1/execute-transaction", transactionHandler.HandleExecuteTransaction)
	mux.HandleFunc("/apiv1/health", healthHandler.HandleHealthCheck)
	
	// ROTA DE TESTE DE EMAIL
	mux.HandleFunc("/apiv1/test-email", healthHandler.HandleTestEmail)

	finalHandler := loggingMiddleware(securityMiddleware(mux))

	srv := &http.Server{
		Addr:    ":8080",
		Handler: finalHandler,
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		slog.Info("Servidor rodando", "port", "8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Erro fatal no servidor", "error", err)
			emailService.SendError(err, map[string]string{"Context": "HTTP Server Crash"})
			os.Exit(1)
		}
	}()

	sig := <-stopChan
	slog.Info("Sinal de encerramento recebido", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	slog.Info("Iniciando desligamento gracioso...")
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Erro ao desligar servidor forçadamente", "error", err)
	}
	slog.Info("Servidor desligado com sucesso.")
}