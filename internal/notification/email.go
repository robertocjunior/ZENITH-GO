package notification

import (
	"crypto/tls"
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

// sendMail é um helper privado que permite ignorar erros de certificado (InsecureSkipVerify)
func (s *EmailService) sendMail(to []string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)

	// 1. Conecta ao servidor SMTP
	conn, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 2. Configura e inicia TLS com SkipVerify
	if ok, _ := conn.Extension("STARTTLS"); ok {
		config := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         s.cfg.SMTPHost,
		}
		if err = conn.StartTLS(config); err != nil {
			return err
		}
	}

	// 3. Autentica
	if s.cfg.SMTPUser != "" && s.cfg.SMTPPass != "" {
		auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)
		if err = conn.Auth(auth); err != nil {
			return err
		}
	}

	// 4. Envia o e-mail (Mail -> Rcpt -> Data)
	if err = conn.Mail(s.cfg.SMTPUser); err != nil {
		return err
	}
	for _, addr := range to {
		if err = conn.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := conn.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}

	return conn.Quit()
}

// SendTestEmail envia um e-mail simples para testar as configurações SMTP (Síncrono)
func (s *EmailService) SendTestEmail(recipient string) error {
	if !s.cfg.EmailEnabled {
		return fmt.Errorf("envio de e-mail está desabilitado no .env")
	}

	subject := "🧪 [Zenith-Go] Teste de Envio de E-mail"
	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; color: #333;">
			<div style="background-color: #00add8; color: white; padding: 15px; border-radius: 5px;">
				<h2 style="margin:0;">Teste de Configuração</h2>
			</div>
			<div style="padding: 20px; background-color: #f9f9f9; border: 1px solid #ddd; margin-top: 10px;">
				<p>Olá,</p>
				<p>Este é um e-mail de teste enviado pela API do <strong>Zenith-Go</strong>.</p>
				<p>Se você recebeu esta mensagem, as configurações SMTP estão corretas! ✅</p>
				<hr>
				<small>Enviado em: %s</small>
			</div>
		</body>
		</html>
	`, time.Now().Format(time.RFC3339))

	// Headers MIME
	headers := make(map[string]string)
	headers["From"] = s.cfg.SMTPUser
	headers["To"] = recipient
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=\"UTF-8\""

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	slog.Info("Enviando e-mail de teste...", "to", recipient, "host", s.cfg.SMTPHost)
	
	// Usa o helper customizado
	if err := s.sendMail([]string{recipient}, []byte(message)); err != nil {
		slog.Error("Falha ao enviar e-mail de teste", "error", err)
		return err
	}

	return nil
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

		// Usa o helper customizado
		if sendErr := s.sendMail(s.cfg.EmailRecipients, []byte(message)); sendErr != nil {
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