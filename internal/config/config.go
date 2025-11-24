package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ApiUrl   string
	AppKey   string
	Token    string
	Username string
	Password string
}

func Load() (*Config, error) {
	// Tenta carregar do arquivo .env, mas não falha se não existir (prod pode usar env vars reais)
	_ = godotenv.Load()

	cfg := &Config{
		ApiUrl:   os.Getenv("SANKHYA_API_URL"),
		AppKey:   os.Getenv("SANKHYA_APPKEY"),
		Token:    os.Getenv("SANKHYA_TOKEN"),
		Username: os.Getenv("SANKHYA_USERNAME"),
		Password: os.Getenv("SANKHYA_PASSWORD"),
	}

	if cfg.ApiUrl == "" || cfg.Username == "" {
		return nil, fmt.Errorf("variáveis de ambiente obrigatórias não preenchidas")
	}

	return cfg, nil
}