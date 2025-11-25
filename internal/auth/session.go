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

// Register salva o token no Redis
func (sm *SessionManager) Register(token string) error {
	ctx := context.Background()
	// Prefixo obrigatório "session:"
	err := sm.client.Set(ctx, "session:"+token, "valid", sm.timeout).Err()
	if err != nil {
		return ErrRedisConnection
	}
	return nil
}

// ValidateAndUpdate verifica e renova
func (sm *SessionManager) ValidateAndUpdate(token string) error {
	ctx := context.Background()
	key := "session:" + token

	_, err := sm.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return ErrSessionExpired
	} else if err != nil {
		return ErrRedisConnection
	}

	sm.client.Expire(ctx, key, sm.timeout)
	return nil
}

// Revoke apaga a sessão
func (sm *SessionManager) Revoke(token string) {
	ctx := context.Background()
	sm.client.Del(ctx, "session:"+token)
}

// CountActiveSessions conta APENAS chaves com prefixo 'session:*'
func (sm *SessionManager) CountActiveSessions() (int64, error) {
	ctx := context.Background()
	
	// Usa Scan para contar chaves com o prefixo correto de forma segura
	// Isso ignora qualquer outra chave que não seja uma sessão de usuário
	var count int64
	iter := sm.client.Scan(ctx, 0, "session:*", 0).Iterator()
	for iter.Next(ctx) {
		count++
	}
	if err := iter.Err(); err != nil {
		return 0, err
	}
	
	return count, nil
}