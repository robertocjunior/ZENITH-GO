package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

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

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Dashboard
	DashboardRefreshRate int

	// E-mail (NOVO)
	EmailEnabled    bool
	EmailRecipients []string
	SMTPHost        string
	SMTPPort        int
	SMTPUser        string
	SMTPPass        string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	logSize, _ := strconv.Atoi(os.Getenv("LOG_MAX_SIZE_MB"))
	logAge, _ := strconv.Atoi(os.Getenv("LOG_MAX_AGE_DAYS"))
	redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	
	dashRefresh, _ := strconv.Atoi(os.Getenv("DASHBOARD_REFRESH_RATE"))
	if dashRefresh <= 0 {
		dashRefresh = 60
	}

	// Parsing de E-mail
	emailEnabled, _ := strconv.ParseBool(os.Getenv("EMAIL_NOTIFICATIONS_ENABLED"))
	smtpPort, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	
	recipientsStr := os.Getenv("EMAIL_RECIPIENTS")
	var recipients []string
	if recipientsStr != "" {
		parts := strings.Split(recipientsStr, ",")
		for _, p := range parts {
			recipients = append(recipients, strings.TrimSpace(p))
		}
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
		// E-mail Configs
		EmailEnabled:    emailEnabled,
		EmailRecipients: recipients,
		SMTPHost:        os.Getenv("SMTP_HOST"),
		SMTPPort:        smtpPort,
		SMTPUser:        os.Getenv("SMTP_USER"),
		SMTPPass:        os.Getenv("SMTP_PASS"),
	}

	if cfg.ApiUrl == "" || cfg.TransactionUrl == "" || cfg.JwtSecret == "" {
		return nil, fmt.Errorf("variáveis de ambiente obrigatórias não preenchidas")
	}

	if cfg.RedisAddr == "" {
		cfg.RedisAddr = "localhost:6379"
	}

	return cfg, nil
}