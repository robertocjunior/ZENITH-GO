package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Config define as opções do logger
type Config struct {
	LogDir     string
	Filename   string
	MaxSize    int  // Megabytes
	MaxBackups int  // Quantidade de arquivos mantidos
	MaxAge     int  // Dias
	Compress   bool // Comprimir logs antigos
}

// Init inicializa o logger global
func Init() {
	// Cria diretório de logs se não existir
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic("não foi possível criar diretório de logs: " + err.Error())
	}

	// Configuração do Lumberjack para rotação de arquivos
	fileWriter := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "zenith.log"),
		MaxSize:    10,   // 10 MB antes de rotacionar
		MaxBackups: 5,    // Manter 5 arquivos antigos
		MaxAge:     28,   // Manter por 28 dias
		Compress:   true, // Comprimir (.gz) arquivos antigos
	}

	// MultiWriter: Escreve no Arquivo E no Console ao mesmo tempo
	multiWriter := io.MultiWriter(os.Stdout, fileWriter)

	// Opções do Handler (JSON estruturado)
	handlerOpts := &slog.HandlerOptions{
		Level: slog.LevelDebug, // Captura tudo de Debug pra cima
		// Customiza a exibição do timestamp
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

	// Cria o Logger
	logger := slog.New(slog.NewJSONHandler(multiWriter, handlerOpts))

	// Define como logger padrão global
	slog.SetDefault(logger)

	slog.Info("Sistema de logs inicializado com sucesso",
		slog.String("dir", logDir),
		slog.Int("max_size_mb", 10),
		slog.Int("retention_days", 28),
	)
}