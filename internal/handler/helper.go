package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"zenith-go/internal/notification"
)

// ErrorMeta estrutura auxiliar para passar contexto do usuário para o erro
type ErrorMeta struct {
	CodUsu    int
	Username  string
	SessionID string // Pode ser o JSESSIONID ou Token
}

// Keys que terão seus valores mascarados no log/email (JSON Payload)
var sensitiveKeys = []string{"password", "senha", "interno", "token", "authorization", "snkjsessionid", "secret", "deviceToken"}

// RespondError centraliza Log + Email + Resposta JSON
func RespondError(w http.ResponseWriter, r *http.Request, notifier *notification.EmailService, code int, msg string, err error, args ...any) {
	errDetails := ""
	if err != nil {
		errDetails = err.Error()
	}

	var meta *ErrorMeta
	var payloadData any

	for _, arg := range args {
		switch v := arg.(type) {
		case ErrorMeta:
			meta = &v
		case *ErrorMeta:
			meta = v
		default:
			payloadData = v
		}
	}

	// 1. LOG NO TERMINAL/ARQUIVO (Comportamento original)
	if code >= 500 {
		logArgs := []any{"error", errDetails, "path", r.URL.Path, "status", code}
		if meta != nil {
			logArgs = append(logArgs, "user", meta.Username, "codusu", meta.CodUsu)
		}
		slog.Error(msg, logArgs...)

		// 2. ENVIA EMAIL (Com o bloco JSON para ferramentas de análise)
		if notifier != nil {
			contextInfo := map[string]string{
				"Path":      r.Method + " " + r.URL.Path,
				"IP":        r.RemoteAddr,
				"Msg":       msg,
				"UserAgent": r.UserAgent(),
			}

			if meta != nil {
				contextInfo["Usuário"] = fmt.Sprintf("%s (Cód: %d)", meta.Username, meta.CodUsu)
				if meta.SessionID != "" {
					contextInfo["SessionID"] = maskID(meta.SessionID)
				}
			}

			// GERAÇÃO DO TRECHO JSON PARA ANÁLISE
			analysisLog := map[string]any{
				"timestamp": time.Now().Format(time.RFC3339),
				"level":     "ERROR",
				"message":   msg,
				"error":     errDetails,
				"request": map[string]any{
					"method": r.Method,
					"path":   r.URL.Path,
					"ip":     r.RemoteAddr,
					"payload": payloadData, // O payload original
				},
				"context": meta,
				"response_code": code,
			}
			
			// Sanitiza o log de análise antes de enviar
			jsonLogString := sanitizeData(analysisLog)
			contextInfo["Analysis_JSON"] = jsonLogString

			notifier.SendError(err, contextInfo)
		}
	} else {
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

// maskID oculta o meio da string (Ex: "ABCDEF123456" -> "ABCD...3456")
func maskID(id string) string {
	const visibleChars = 4
	if len(id) <= (visibleChars * 2) {
		return "*****" // Muito curto para mostrar partes
	}
	return fmt.Sprintf("%s...%s", id[:visibleChars], id[len(id)-visibleChars:])
}

// sanitizeData converte o objeto para JSON, mascara chaves sensíveis e retorna string formatada
func sanitizeData(data any) string {
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("Erro ao serializar payload: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return string(bytes)
	}

	maskSensitive(raw)
	prettyBytes, _ := json.MarshalIndent(raw, "", "  ")
	return string(prettyBytes)
}

func maskSensitive(m map[string]any) {
	for k, v := range m {
		keyLower := strings.ToLower(k)
		isSensitive := false
		for _, sens := range sensitiveKeys {
			if strings.Contains(keyLower, sens) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			m[k] = "*****"
			continue
		}

		if nestedMap, ok := v.(map[string]any); ok {
			maskSensitive(nestedMap)
		} else if nestedInterface, ok := v.(map[string]interface{}); ok {
			maskSensitive(nestedInterface)
		}
	}
}