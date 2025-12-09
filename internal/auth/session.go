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

const (
	KeepAliveKey = "sessions:keepalive" // Nome do Sorted Set
	// ALTERADO: Tempo reduzido para 15 segundos conforme padrão Sankhya
	KeepAliveInterval = 15 * time.Second
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

// Register salva token e agenda o primeiro keep-alive
func (sm *SessionManager) Register(token string, snkSessionID string) error {
	ctx := context.Background()
	pipe := sm.client.Pipeline()

	// 1. Salva a sessão normal
	pipe.Set(ctx, "session:"+token, snkSessionID, sm.timeout)
	
	// 2. Agenda o próximo keep-alive
	nextPing := float64(time.Now().Add(KeepAliveInterval).Unix())
	pipe.ZAdd(ctx, KeepAliveKey, redis.Z{
		Score:  nextPing,
		Member: token,
	})

	_, err := pipe.Exec(ctx)
	if err != nil {
		return ErrRedisConnection
	}
	return nil
}

// ValidateAndUpdate renova sessão e posterga o keep-alive (pois o usuário está ativo)
func (sm *SessionManager) ValidateAndUpdate(token string) error {
	ctx := context.Background()
	key := "session:" + token

	_, err := sm.client.Get(ctx, key).Result()
	if err == redis.Nil {
		sm.client.ZRem(ctx, KeepAliveKey, token) // Remove do agendamento se expirou
		return ErrSessionExpired
	} else if err != nil {
		return ErrRedisConnection
	}

	pipe := sm.client.Pipeline()
	
	// Renova expiração da chave principal
	pipe.Expire(ctx, key, sm.timeout)
	
	// RE-AGENDA
	nextPing := float64(time.Now().Add(KeepAliveInterval).Unix())
	pipe.ZAdd(ctx, KeepAliveKey, redis.Z{
		Score:  nextPing,
		Member: token,
	})

	_, err = pipe.Exec(ctx) 
	return err // Retorna erro se falhar
}

// GetSankhyaSession recupera o JSessionID vinculado ao token JWT
func (sm *SessionManager) GetSankhyaSession(token string) (string, error) {
	ctx := context.Background()
	key := "session:" + token
	val, err := sm.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrSessionExpired
	}
	return val, err
}

// Revoke apaga a sessão e remove do agendamento
func (sm *SessionManager) Revoke(token string) {
	ctx := context.Background()
	pipe := sm.client.Pipeline()
	pipe.Del(ctx, "session:"+token)
	pipe.ZRem(ctx, KeepAliveKey, token)
	pipe.Exec(ctx)
}

// CountActiveSessions conta sessões ativas
func (sm *SessionManager) CountActiveSessions() (int64, error) {
	ctx := context.Background()
	var count int64
	iter := sm.client.Scan(ctx, 0, "session:*", 0).Iterator()
	for iter.Next(ctx) {
		count++
	}
	return count, iter.Err()
}

// --- MÉTODOS PARA O WORKER DE KEEP-ALIVE ---

// GetTokensToPing busca tokens cujo horário de ping (score) é menor ou igual a Agora
func (sm *SessionManager) GetTokensToPing() ([]string, error) {
	ctx := context.Background()
	now := float64(time.Now().Unix())
	
	// ZRangeByScore busca elementos com score entre -infinito e Agora
	tokens, err := sm.client.ZRangeByScore(ctx, KeepAliveKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%f", now),
	}).Result()

	return tokens, err
}

// UpdatePingTime atualiza o horário do próximo ping para um token específico
func (sm *SessionManager) UpdatePingTime(token string) error {
	ctx := context.Background()
	// Verifica se a sessão ainda existe antes de reagendar
	exists, err := sm.client.Exists(ctx, "session:"+token).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		// Se a sessão morreu, remove do ZSET
		sm.client.ZRem(ctx, KeepAliveKey, token)
		return nil
	}

	nextPing := float64(time.Now().Add(KeepAliveInterval).Unix())
	return sm.client.ZAdd(ctx, KeepAliveKey, redis.Z{
		Score:  nextPing,
		Member: token,
	}).Err()
}