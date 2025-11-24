package auth

import "github.com/google/uuid"

// NewDeviceToken gera um identificador único universal (UUID v4).
// Sua aleatoriedade garante que não haverá colisão, dispensando verificação no BD.
func NewDeviceToken() string {
	return uuid.NewString()
}