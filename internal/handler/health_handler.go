package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"time"
	"zenith-go/internal/auth"
	"zenith-go/internal/config"
)

type HealthHandler struct {
	Session *auth.SessionManager
	Config  *config.Config // Injeção de dependência
}

type HealthResponse struct {
	NodeID         string `json:"node_id"`
	Status         string `json:"status"`
	UptimeSeconds  int64  `json:"uptime_seconds"`
	MemoryUsageMB  uint64 `json:"memory_usage_mb"`
	Goroutines     int    `json:"goroutines"`
	RedisStatus    string `json:"redis_status"`
	ActiveSessions int64  `json:"active_sessions"`
	RefreshRate    int    `json:"refresh_rate"` // Novo campo para o front
	Timestamp      string `json:"timestamp"`
}

var startTime = time.Now()

func (h *HealthHandler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	redisStatus := "connected"
	sessions, err := h.Session.CountActiveSessions()
	if err != nil {
		redisStatus = "disconnected: " + err.Error()
		sessions = -1
	}

	resp := HealthResponse{
		NodeID:         hostname,
		Status:         "online",
		UptimeSeconds:  int64(time.Since(startTime).Seconds()),
		MemoryUsageMB:  m.Alloc / 1024 / 1024,
		Goroutines:     runtime.NumGoroutine(),
		RedisStatus:    redisStatus,
		ActiveSessions: sessions,
		RefreshRate:    h.Config.DashboardRefreshRate, // Envia valor do .env
		Timestamp:      time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(resp)
}