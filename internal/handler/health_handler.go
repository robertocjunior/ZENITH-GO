package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"time"
	"zenith-go/internal/auth"
)

type HealthHandler struct {
	Session *auth.SessionManager
}

type HealthResponse struct {
	NodeID         string `json:"node_id"`         // Nome do Container (ex: zenith-app-1)
	Status         string `json:"status"`          // "online"
	Uptime         string `json:"uptime"`          // Desde quando está rodando
	MemoryUsageMB  uint64 `json:"memory_usage_mb"` // Memória RAM usada
	Goroutines     int    `json:"goroutines"`      // Threads leves do Go
	RedisStatus    string `json:"redis_status"`    // "connected" ou erro
	ActiveSessions int64  `json:"active_sessions"` // Total de sessões no Redis
	Timestamp      string `json:"timestamp"`
}

var startTime = time.Now()

func (h *HealthHandler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// 1. Coleta dados do Sistema
	hostname, _ := os.Hostname()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 2. Coleta dados do Redis
	redisStatus := "connected"
	sessions, err := h.Session.CountActiveSessions()
	if err != nil {
		redisStatus = "disconnected: " + err.Error()
		sessions = -1 // Indica erro na contagem
	}

	resp := HealthResponse{
		NodeID:         hostname,
		Status:         "online",
		Uptime:         time.Since(startTime).String(),
		MemoryUsageMB:  m.Alloc / 1024 / 1024,
		Goroutines:     runtime.NumGoroutine(),
		RedisStatus:    redisStatus,
		ActiveSessions: sessions,
		Timestamp:      time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	// Headers para permitir que o painel leia (CORS)
	w.Header().Set("Access-Control-Allow-Origin", "*") 
	json.NewEncoder(w).Encode(resp)
}