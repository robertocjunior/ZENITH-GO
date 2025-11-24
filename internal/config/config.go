package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ApiUrl    string
	AppKey    string
	Token     string
	Username  string
	Password  string
	JwtSecret string // Novo campo
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		ApiUrl:    os.Getenv("SANKHYA_API_URL"),
		AppKey:    os.Getenv("SANKHYA_APPKEY"),
		Token:     os.Getenv("SANKHYA_TOKEN"),
		Username:  os.Getenv("SANKHYA_USERNAME"),
		Password:  os.Getenv("SANKHYA_PASSWORD"),
		JwtSecret: os.Getenv("JWT_SECRET"), // Carrega o segredo
	}

	if cfg.ApiUrl == "" || cfg.JwtSecret == "" {
		return nil, fmt.Errorf("variáveis de ambiente obrigatórias não preenchidas")
	}

	return cfg, nil
}