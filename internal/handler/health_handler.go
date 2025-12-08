package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"time"
	"zenith-go/internal/auth"
	"zenith-go/internal/config"
	"zenith-go/internal/notification" // Import necessário
)

type HealthHandler struct {
	Session  *auth.SessionManager
	Config   *config.Config
	Notifier *notification.EmailService // Novo campo para injeção
}

type HealthResponse struct {
	NodeID         string `json:"node_id"`
	Status         string `json:"status"`
	UptimeSeconds  int64  `json:"uptime_seconds"`
	MemoryUsageMB  uint64 `json:"memory_usage_mb"`
	Goroutines     int    `json:"goroutines"`
	RedisStatus    string `json:"redis_status"`
	ActiveSessions int64  `json:"active_sessions"`
	RefreshRate    int    `json:"refresh_rate"`
	Timestamp      string `json:"timestamp"`
}

type testEmailInput struct {
	Email string `json:"email"`
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
		RefreshRate:    h.Config.DashboardRefreshRate,
		Timestamp:      time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(resp)
}

// HandleTestEmail processa o envio de e-mail de teste
func (h *HealthHandler) HandleTestEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var input testEmailInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	if input.Email == "" {
		RespondError(w, r, h.Notifier, http.StatusBadRequest, "O campo 'email' é obrigatório", nil)
		return
	}

	// Tenta enviar o e-mail
	if err := h.Notifier.SendTestEmail(input.Email); err != nil {
		// Retorna 500 se falhar no SMTP
		RespondError(w, r, h.Notifier, http.StatusInternalServerError, "Falha ao enviar e-mail de teste: "+err.Error(), err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "E-mail de teste enviado com sucesso para " + input.Email,
	})
}