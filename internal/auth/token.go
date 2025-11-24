package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateToken cria um JWT contendo username e codusu
func GenerateToken(username string, codUsu int, secret string) (string, error) {
	claims := jwt.MapClaims{
		"username": username,
		"codusu":   codUsu,
		"exp":      time.Now().Add(50 * time.Minute).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken verifica a assinatura e retorna o CODUSU se válido
func ValidateToken(tokenString string, secret string) (int, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Valida o método de assinatura
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de assinatura inesperado: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return 0, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Extrai o codusu (JSON numbers vêm como float64)
		codUsuFloat, ok := claims["codusu"].(float64)
		if !ok {
			return 0, fmt.Errorf("codusu não encontrado no token")
		}
		return int(codUsuFloat), nil
	}

	return 0, fmt.Errorf("token inválido")
}