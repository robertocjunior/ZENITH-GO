package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
	"zenith-go/internal/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Init inicializa o logger global com base na configuração
func Init(cfg *config.Config) {
	// Cria diretório de logs se não existir
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic("não foi possível criar diretório de logs: " + err.Error())
	}

	// Lógica de Precedência e Defaults:
	// 1. Se o usuário definiu LOG_MAX_SIZE_MB, usa. Caso contrário, padrão é 100MB.
	// 2. Se o usuário definiu LOG_MAX_AGE_DAYS, usa. Caso contrário, 0 (Lumberjack não remove por idade).
	
	maxSize := 100 // Default padrão exigido: 100MB
	if cfg.LogMaxSize > 0 {
		maxSize = cfg.LogMaxSize
	}

	maxAge := 0 // Default: sem limite de dias (controlado apenas por tamanho/backups)
	if cfg.LogMaxAge > 0 {
		maxAge = cfg.LogMaxAge
	}

	// Configuração do Lumberjack para rotação
	fileWriter := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "zenith.log"),
		MaxSize:    maxSize,    // MB
		MaxAge:     maxAge,     // Dias
		MaxBackups: 5,          // Mantém até 5 arquivos antigos rotacionados
		Compress:   true,       // Comprimir arquivos antigos (.gz)
		LocalTime:  true,       // Usa horário local no nome do arquivo
	}

	// MultiWriter: Escreve no Arquivo E no Console
	multiWriter := io.MultiWriter(os.Stdout, fileWriter)

	// Handler JSON Estruturado
	handlerOpts := &slog.HandlerOptions{
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
	}

	logger := slog.New(slog.NewJSONHandler(multiWriter, handlerOpts))
	slog.SetDefault(logger)

	slog.Info("Sistema de logs inicializado",
		slog.String("dir", logDir),
		slog.Int("max_size_mb", maxSize),
		slog.Int("max_age_days", maxAge),
	)
}