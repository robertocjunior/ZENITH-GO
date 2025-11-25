package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrSessionExpired  = errors.New("sessão expirada por inatividade")
	ErrSessionNotFound = errors.New("sessão inválida ou servidor reiniciado")
	ErrRedisConnection = errors.New("erro de conexão com o banco de sessões")
)

type SessionManager struct {
	client  *redis.Client
	timeout time.Duration
}

// NewSessionManager inicializa a conexão com o Redis
func NewSessionManager(addr, password string, db int, timeoutMinutes int) (*SessionManager, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Teste de conexão (Ping)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("falha ao conectar no Redis: %w", err)
	}

	return &SessionManager{
		client:  rdb,
		timeout: time.Duration(timeoutMinutes) * time.Minute,
	}, nil
}

// Register salva o token no Redis com tempo de expiração
func (sm *SessionManager) Register(token string) error {
	ctx := context.Background()
	// A chave será "session:{token}". O valor é "valid" (ou poderia ser o ID do usuário).
	// O TTL (Time To Live) é definido pelo timeout.
	err := sm.client.Set(ctx, "session:"+token, "valid", sm.timeout).Err()
	if err != nil {
		return ErrRedisConnection
	}
	return nil
}

// ValidateAndUpdate verifica se a sessão existe no Redis.
// Se existir, reinicia a contagem do tempo de expiração (Sliding Expiration).
func (sm *SessionManager) ValidateAndUpdate(token string) error {
	ctx := context.Background()
	key := "session:" + token

	// Verifica se existe (GET)
	_, err := sm.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return ErrSessionExpired // Chave não existe (expirou ou nunca existiu)
	} else if err != nil {
		return ErrRedisConnection
	}

	// Renova o tempo de vida (EXPIRE)
	sm.client.Expire(ctx, key, sm.timeout)
	return nil
}

// Revoke apaga a sessão do Redis imediatamente (Logout)
func (sm *SessionManager) Revoke(token string) {
	ctx := context.Background()
	// Não precisamos tratar erro no logout, apenas tentar apagar
	sm.client.Del(ctx, "session:"+token)
}