package logger

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"
	"zenith-go/internal/config"

	"github.com/lmittmann/tint"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Init inicializa o logger global com base na configuração
func Init(cfg *config.Config) {
	// Cria diretório de logs se não existir
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic("não foi possível criar diretório de logs: " + err.Error())
	}

	maxSize := 100
	if cfg.LogMaxSize > 0 {
		maxSize = cfg.LogMaxSize
	}

	maxAge := 0
	if cfg.LogMaxAge > 0 {
		maxAge = cfg.LogMaxAge
	}

	// 1. Configuração do Arquivo (JSON Estruturado - Máquina)
	fileWriter := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "zenith.log"),
		MaxSize:    maxSize,
		MaxAge:     maxAge,
		MaxBackups: 5,
		Compress:   true,
		LocalTime:  true,
	}

	// Opções para o arquivo (JSON completo com timestamp ISO)
	fileHandler := slog.NewJSONHandler(fileWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "time",
					Value: slog.StringValue(a.Value.Time().Format(time.RFC3339)),
				}
			}
			return a
		},
	})

	// 2. Configuração do Console (Tint - Humano/Colorido)
	consoleHandler := tint.NewHandler(os.Stdout, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: time.TimeOnly, // Mostra apenas 15:04:05 (mais limpo)
		AddSource:  false,         // Não polui o terminal com o nome do arquivo fonte
	})

	// 3. Unifica os dois (Fanout)
	multiHandler := NewFanoutHandler(consoleHandler, fileHandler)

	// Define como logger padrão
	logger := slog.New(multiHandler)
	slog.SetDefault(logger)

	slog.Info("Sistema de logs inicializado",
		"dir", logDir,
		"mode", "Hybrid (Pretty Console + JSON File)",
	)
}

// --- Fanout Handler (Distribui o log para múltiplos handlers) ---

type FanoutHandler struct {
	handlers []slog.Handler
}

func NewFanoutHandler(handlers ...slog.Handler) *FanoutHandler {
	return &FanoutHandler{handlers: handlers}
}

// Enabled reporta se o nível está habilitado (se qualquer um dos handlers aceitar)
func (h *FanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle despacha o record para todos os handlers
func (h *FanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			// O método Handle pode modificar o record, então é seguro clonar se necessário,
			// mas o slog.Record é passado por valor, então geralmente é seguro.
			_ = handler.Handle(ctx, r)
		}
	}
	return nil
}

// WithAttrs retorna um novo FanoutHandler com atributos adicionados a todos os filhos
func (h *FanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithAttrs(attrs)
	}
	return NewFanoutHandler(newHandlers...)
}

// WithGroup retorna um novo FanoutHandler com grupo adicionado a todos os filhos
func (h *FanoutHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithGroup(name)
	}
	return NewFanoutHandler(newHandlers...)
}