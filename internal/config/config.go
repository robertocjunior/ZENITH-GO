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
	
	// Configurações de Log
	LogMaxSize int 
	LogMaxAge  int

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Dashboard
	DashboardRefreshRate int // Segundos
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	logSize, _ := strconv.Atoi(os.Getenv("LOG_MAX_SIZE_MB"))
	logAge, _ := strconv.Atoi(os.Getenv("LOG_MAX_AGE_DAYS"))
	redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	
	// Leitura do Refresh Rate
	dashRefresh, _ := strconv.Atoi(os.Getenv("DASHBOARD_REFRESH_RATE"))
	if dashRefresh <= 0 {
		dashRefresh = 60 // Padrão: 1 minuto (60 segundos)
	}

	cfg := &Config{
		ApiUrl:               os.Getenv("SANKHYA_API_URL"),
		TransactionUrl:       os.Getenv("SANKHYA_TRANSACTION_URL"),
		AppKey:               os.Getenv("SANKHYA_APPKEY"),
		Token:                os.Getenv("SANKHYA_TOKEN"),
		Username:             os.Getenv("SANKHYA_USERNAME"),
		Password:             os.Getenv("SANKHYA_PASSWORD"),
		JwtSecret:            os.Getenv("JWT_SECRET"),
		LogMaxSize:           logSize,
		LogMaxAge:            logAge,
		RedisAddr:            os.Getenv("REDIS_ADDR"),
		RedisPassword:        os.Getenv("REDIS_PASSWORD"),
		RedisDB:              redisDB,
		DashboardRefreshRate: dashRefresh,
	}

	if cfg.ApiUrl == "" || cfg.TransactionUrl == "" || cfg.JwtSecret == "" {
		return nil, fmt.Errorf("variáveis de ambiente obrigatórias não preenchidas")
	}

	if cfg.RedisAddr == "" {
		cfg.RedisAddr = "localhost:6379"
	}

	return cfg, nil
}