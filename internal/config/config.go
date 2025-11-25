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

	// Logs
	LogMaxSize int
	LogMaxAge  int

	// Redis (NOVO)
	RedisAddr     string
	RedisPassword string
	RedisDB       int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	logSize, _ := strconv.Atoi(os.Getenv("LOG_MAX_SIZE_MB"))
	logAge, _ := strconv.Atoi(os.Getenv("LOG_MAX_AGE_DAYS"))
	redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB"))

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
		// Redis Defaults
		RedisAddr:     os.Getenv("REDIS_ADDR"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       redisDB,
	}

	// Validações básicas
	if cfg.ApiUrl == "" || cfg.JwtSecret == "" {
		return nil, fmt.Errorf("variáveis obrigatórias (API_URL, JWT_SECRET) não preenchidas")
	}

	// Fallback para Redis local se não definido (para dev sem docker)
	if cfg.RedisAddr == "" {
		cfg.RedisAddr = "localhost:6379"
	}

	return cfg, nil
}