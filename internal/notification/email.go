package notification

import (
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"
	"zenith-go/internal/config"
)

type EmailService struct {
	cfg *config.Config
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{cfg: cfg}
}

// SendError envia um alerta de erro por e-mail (Assíncrono)
func (s *EmailService) SendError(err error, contextInfo map[string]string) {
	if !s.cfg.EmailEnabled || len(s.cfg.EmailRecipients) == 0 {
		return
	}

	// Dispara em goroutine para não bloquear a API
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Panic recuperado ao enviar e-mail", "recover", r)
			}
		}()

		subject := fmt.Sprintf("🚨 [Zenith-Go] Erro Crítico: %s", truncate(err.Error(), 50))
		body := buildHtmlBody(err, contextInfo)
		
		// Headers MIME
		headers := make(map[string]string)
		headers["From"] = s.cfg.SMTPUser
		headers["To"] = strings.Join(s.cfg.EmailRecipients, ",")
		headers["Subject"] = subject
		headers["MIME-Version"] = "1.0"
		headers["Content-Type"] = "text/html; charset=\"UTF-8\""

		message := ""
		for k, v := range headers {
			message += fmt.Sprintf("%s: %s\r\n", k, v)
		}
		message += "\r\n" + body

		addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
		auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)

		if sendErr := smtp.SendMail(addr, auth, s.cfg.SMTPUser, s.cfg.EmailRecipients, []byte(message)); sendErr != nil {
			slog.Error("Falha ao enviar e-mail de notificação", "error", sendErr)
		} else {
			slog.Info("E-mail de notificação enviado com sucesso")
		}
	}()
}

func buildHtmlBody(err error, info map[string]string) string {
	listItems := ""
	for k, v := range info {
		listItems += fmt.Sprintf("<li><strong>%s:</strong> %s</li>", k, v)
	}

	return fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; color: #333;">
			<div style="background-color: #d32f2f; color: white; padding: 15px; border-radius: 5px;">
				<h2 style="margin:0;">Erro no Sistema Zenith-Go</h2>
			</div>
			<div style="padding: 20px; background-color: #f9f9f9; border: 1px solid #ddd; margin-top: 10px;">
				<h3>Erro:</h3>
				<p style="background-color: #fff; padding: 10px; border-left: 4px solid #d32f2f; font-family: monospace;">
					%s
				</p>
				<h3>Contexto:</h3>
				<ul>
					<li><strong>Data:</strong> %s</li>
					%s
				</ul>
			</div>
		</body>
		</html>
	`, err.Error(), time.Now().Format(time.RFC3339), listItems)
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}