package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	ApiUrl         string
	TransactionUrl string
	AppKey         string
	Token          string
	Username       string
	Password       string
	JwtSecret      string
	
	// Configurações de Log (Opcionais)
	LogMaxSize int // em MB
	LogMaxAge  int // em Dias
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	// Leitura de configurações opcionais de Log (ignora erro de conversão, assume 0)
	logSize, _ := strconv.Atoi(os.Getenv("LOG_MAX_SIZE_MB"))
	logAge, _ := strconv.Atoi(os.Getenv("LOG_MAX_AGE_DAYS"))

	cfg := &Config{
		ApiUrl:         os.Getenv("SANKHYA_API_URL"),
		TransactionUrl: os.Getenv("SANKHYA_TRANSACTION_URL"),
		AppKey:         os.Getenv("SANKHYA_APPKEY"),
		Token:          os.Getenv("SANKHYA_TOKEN"),
		Username:       os.Getenv("SANKHYA_USERNAME"),
		Password:       os.Getenv("SANKHYA_PASSWORD"),
		JwtSecret:      os.Getenv("JWT_SECRET"),
		LogMaxSize:     logSize,
		LogMaxAge:      logAge,
	}

	if cfg.ApiUrl == "" || cfg.TransactionUrl == "" || cfg.JwtSecret == "" {
		return nil, fmt.Errorf("variáveis de ambiente obrigatórias não preenchidas")
	}

	return cfg, nil
}