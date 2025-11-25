package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ApiUrl         string
	TransactionUrl string // Novo campo
	AppKey         string
	Token          string
	Username       string
	Password       string
	JwtSecret      string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		ApiUrl:         os.Getenv("SANKHYA_API_URL"),
		TransactionUrl: os.Getenv("SANKHYA_TRANSACTION_URL"), // Carrega do .env
		AppKey:         os.Getenv("SANKHYA_APPKEY"),
		Token:          os.Getenv("SANKHYA_TOKEN"),
		Username:       os.Getenv("SANKHYA_USERNAME"),
		Password:       os.Getenv("SANKHYA_PASSWORD"),
		JwtSecret:      os.Getenv("JWT_SECRET"),
	}

	if cfg.ApiUrl == "" || cfg.TransactionUrl == "" || cfg.JwtSecret == "" {
		return nil, fmt.Errorf("variáveis de ambiente obrigatórias não preenchidas (incluindo SANKHYA_TRANSACTION_URL)")
	}

	return cfg, nil
}