package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"zenith-go/internal/notification"
)

// Keys que terão seus valores mascarados no log/email
var sensitiveKeys = []string{"password", "senha", "interno", "token", "authorization", "snkjsessionid", "secret", "deviceToken"}

// RespondError centraliza Log + Email + Resposta JSON
// Agora aceita um argumento opcional 'payload' (variadic) para incluir os dados que causaram o erro.
func RespondError(w http.ResponseWriter, r *http.Request, notifier *notification.EmailService, code int, msg string, err error, payload ...any) {
	errDetails := ""
	if err != nil {
		errDetails = err.Error()
	}

	// 1. LOG NO TERMINAL/ARQUIVO
	if code >= 500 {
		slog.Error(msg, "error", errDetails, "path", r.URL.Path, "status", code)

		// 2. ENVIA EMAIL (APENAS ERROS 5xx)
		if notifier != nil {
			contextInfo := map[string]string{
				"Path":      r.Method + " " + r.URL.Path,
				"IP":        r.RemoteAddr,
				"Msg":       msg,
				"UserAgent": r.UserAgent(),
			}

			// Se um payload foi passado, sanitiza e adiciona ao contexto
			if len(payload) > 0 && payload[0] != nil {
				sanitizedPayload := sanitizeData(payload[0])
				contextInfo["Payload"] = sanitizedPayload
			}

			notifier.SendError(err, contextInfo)
		}
	} else {
		// Erros 4xx são apenas avisos
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

// sanitizeData converte o objeto para JSON, mascara chaves sensíveis e retorna string formatada
func sanitizeData(data any) string {
	// Converte para JSON para normalizar structs e maps
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("Erro ao serializar payload: %v", err)
	}

	var raw map[string]any
	// Se não for um objeto JSON (ex: string direta ou array), retorna como está
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return string(bytes)
	}

	// Mascara recursivamente
	maskSensitive(raw)

	// Formata bonito (Pretty Print)
	prettyBytes, _ := json.MarshalIndent(raw, "", "  ")
	return string(prettyBytes)
}

func maskSensitive(m map[string]any) {
	for k, v := range m {
		// Verifica se a chave é sensível
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

		// Recurso para objetos aninhados (ex: map dentro de map)
		if nestedMap, ok := v.(map[string]any); ok {
			maskSensitive(nestedMap)
		} else if nestedInterface, ok := v.(map[string]interface{}); ok {
			maskSensitive(nestedInterface)
		}
	}
}