package auth

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrSessionExpired  = errors.New("sessão expirada por inatividade")
	ErrSessionNotFound = errors.New("sessão inválida ou servidor reiniciado")
)

type SessionManager struct {
	sessions sync.Map // Armazena token -> time.Time (última atividade)
	timeout  time.Duration
}

// NewSessionManager cria o gerenciador e inicia a limpeza automática em background
func NewSessionManager(timeoutMinutes int) *SessionManager {
	sm := &SessionManager{
		timeout: time.Duration(timeoutMinutes) * time.Minute,
	}

	// Garbage Collector: Remove sessões mortas a cada 5 minutos para economizar RAM
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			now := time.Now()
			sm.sessions.Range(func(key, value any) bool {
				lastSeen := value.(time.Time)
				if now.Sub(lastSeen) > sm.timeout {
					sm.sessions.Delete(key)
				}
				return true
			})
		}
	}()

	return sm
}

// Register inicia o rastreamento de uma nova sessão
func (sm *SessionManager) Register(token string) {
	sm.sessions.Store(token, time.Now())
}

// ValidateAndUpdate verifica se a sessão existe e não expirou. Se válida, renova o tempo.
func (sm *SessionManager) ValidateAndUpdate(token string) error {
	lastSeenAny, ok := sm.sessions.Load(token)
	if !ok {
		return ErrSessionNotFound // Retorna erro se o servidor reiniciou (memória limpa)
	}

	lastSeen := lastSeenAny.(time.Time)
	if time.Since(lastSeen) > sm.timeout {
		sm.sessions.Delete(token)
		return ErrSessionExpired
	}

	// Renovação da sessão (Sliding Window)
	sm.sessions.Store(token, time.Now())
	return nil
}

// Revoke remove a sessão manualmente (Logout)
func (sm *SessionManager) Revoke(token string) {
	sm.sessions.Delete(token)
}