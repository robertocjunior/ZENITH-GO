package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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

	// Separa o Payload do MetaData nos argumentos opcionais
	var meta *ErrorMeta
	var payloadData any

	for _, arg := range args {
		switch v := arg.(type) {
		case ErrorMeta:
			meta = &v
		case *ErrorMeta:
			meta = v
		default:
			// Assume que qualquer outra coisa é o payload da requisição
			payloadData = v
		}
	}

	// 1. LOG NO TERMINAL/ARQUIVO
	if code >= 500 {
		logArgs := []any{"error", errDetails, "path", r.URL.Path, "status", code}
		if meta != nil {
			// No Log do servidor mantemos informações úteis para o sysadmin
			logArgs = append(logArgs, "user", meta.Username, "codusu", meta.CodUsu)
		}
		slog.Error(msg, logArgs...)

		// 2. ENVIA EMAIL (APENAS ERROS 5xx)
		if notifier != nil {
			contextInfo := map[string]string{
				"Path":      r.Method + " " + r.URL.Path,
				"IP":        r.RemoteAddr,
				"Msg":       msg,
				"UserAgent": r.UserAgent(),
			}

			// Adiciona contexto do usuário se disponível
			if meta != nil {
				contextInfo["Usuário"] = fmt.Sprintf("%s (Cód: %d)", meta.Username, meta.CodUsu)
				if meta.SessionID != "" {
					// ALTERAÇÃO: Aplica a máscara no SessionID para o E-mail
					contextInfo["SessionID"] = maskID(meta.SessionID)
				}
			}

			// Adiciona payload sanitizado se disponível
			if payloadData != nil {
				sanitizedPayload := sanitizeData(payloadData)
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