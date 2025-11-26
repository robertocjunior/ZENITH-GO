package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"zenith-go/internal/notification"
)

// RespondError centraliza Log + Email + Resposta JSON
func RespondError(w http.ResponseWriter, r *http.Request, notifier *notification.EmailService, code int, msg string, err error) {
	errDetails := ""
	if err != nil {
		errDetails = err.Error()
	}

	// 1. LOG NO TERMINAL/ARQUIVO (ISSO MANTÉM O COMPORTAMENTO ORIGINAL)
	if code >= 500 {
		slog.Error(msg, "error", errDetails, "path", r.URL.Path, "status", code)
		
		// 2. ENVIA EMAIL (APENAS ERROS 5xx)
		if notifier != nil {
			contextInfo := map[string]string{
				"Path": r.Method + " " + r.URL.Path,
				"IP":   r.RemoteAddr,
				"Msg":  msg,
			}
			notifier.SendError(err, contextInfo)
		}
	} else {
		// Erros 4xx são avisos
		slog.Warn(msg, "error", errDetails, "path", r.URL.Path, "status", code)
	}

	// 3. RESPOSTA JSON PARA O CLIENTE
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   msg,
		"details": errDetails,
	})
}