package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateToken cria um JWT válido por 50 minutos
func GenerateToken(username string, secret string) (string, error) {
	claims := jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(50 * time.Minute).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}