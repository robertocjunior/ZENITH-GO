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
	NodeID         string `json:"node_id"`
	Status         string `json:"status"`
	UptimeSeconds  int64  `json:"uptime_seconds"` // MUDANÇA: Envia segundos puros
	MemoryUsageMB  uint64 `json:"memory_usage_mb"`
	Goroutines     int    `json:"goroutines"`
	RedisStatus    string `json:"redis_status"`
	ActiveSessions int64  `json:"active_sessions"`
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
		// Calcula segundos desde o início
		UptimeSeconds:  int64(time.Since(startTime).Seconds()), 
		MemoryUsageMB:  m.Alloc / 1024 / 1024,
		Goroutines:     runtime.NumGoroutine(),
		RedisStatus:    redisStatus,
		ActiveSessions: sessions,
		Timestamp:      time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(resp)
}