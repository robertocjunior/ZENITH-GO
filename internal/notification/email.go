package notification

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/smtp"
	"sort"
	"strings"
	"time"
	"zenith-go/internal/config"
)

// Link direto para a logo (Raw GitHub) - Melhor compatibilidade com Gmail/Outlook
const zenithLogoUrl = "https://raw.githubusercontent.com/robertocjunior/ZENITH-GO/main/docs/zenith.svg"

type EmailService struct {
	cfg *config.Config
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{cfg: cfg}
}

// sendMail helper privado com InsecureSkipVerify (para hosts compartilhados)
func (s *EmailService) sendMail(to []string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)

	conn, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	if ok, _ := conn.Extension("STARTTLS"); ok {
		config := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         s.cfg.SMTPHost,
		}
		if err = conn.StartTLS(config); err != nil {
			return err
		}
	}

	if s.cfg.SMTPUser != "" && s.cfg.SMTPPass != "" {
		auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)
		if err = conn.Auth(auth); err != nil {
			return err
		}
	}

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

// getHtmlTemplate monta o e-mail responsivo padrão Zenith-Go
func (s *EmailService) getHtmlTemplate(title string, bodyContent string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #f4f7f6; color: #333; margin: 0; padding: 20px; }
        .container { max-width: 800px; margin: 0 auto; background-color: #ffffff; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 15px rgba(0,0,0,0.05); }
        .header { background-color: #00529B; padding: 30px 20px; text-align: center; }
        .header img { max-width: 220px; height: auto; margin-bottom: 15px; }
        .header h1 { color: #ffffff; margin: 10px 0 0 0; font-size: 24px; }
        .content { padding: 30px; }
        .content h2 { color: #D32F2F; font-size: 20px; border-bottom: 2px solid #f0f0f0; padding-bottom: 10px; margin-top: 0; }
        .details-grid { display: grid; grid-template-columns: 150px 1fr; gap: 10px 20px; margin-top: 20px; font-size: 14px; }
        .details-grid dt { font-weight: bold; color: #555; }
        .details-grid dd { margin: 0; background-color: #f9f9f9; padding: 8px; border-radius: 6px; word-break: break-all; }
        .details-grid dd.env-prod { background-color: #ffebee; color: #c62828; font-weight: bold; border: 1px solid #ffcdd2; }
        .code-block { margin-top: 30px; }
        .code-block h3 { font-size: 16px; color: #333; margin-bottom: 10px; }
        .code-block pre { background-color: #2d2d2d; color: #f2f2f2; padding: 15px; border-radius: 6px; white-space: pre-wrap; word-wrap: break-word; font-family: 'Courier New', Courier, monospace; font-size: 13px; }
        .footer { background-color: #eef2f5; color: #888; font-size: 12px; text-align: center; padding: 20px; }
        code { font-family: 'Courier New', Courier, monospace; }
        @media (max-width: 600px) { .details-grid { grid-template-columns: 1fr; } .content { padding: 20px; } }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <img src="%s" alt="Zenith WMS Logo" />
            <h1>%s</h1>
        </div>
        <div class="content">
            %s
        </div>
        <div class="footer">
            Esta é uma notificação automática do Zenith-Go. Por favor, não responda a este e-mail.
        </div>
    </div>
</body>
</html>`, zenithLogoUrl, title, bodyContent)
}

// SendTestEmail envia e-mail de teste formatado
func (s *EmailService) SendTestEmail(recipient string) error {
	if !s.cfg.EmailEnabled {
		return fmt.Errorf("envio de e-mail desabilitado no .env")
	}

	title := "Teste de Configuração"
	subject := "🧪 [Zenith-Go] Teste de Envio de E-mail"

	// Conteúdo do corpo
	bodyContent := fmt.Sprintf(`
		<h2 style="color: #00add8; border-bottom-color: #00add8;">Configuração Validada com Sucesso</h2>
		<p>Olá,</p>
		<p>Este é um e-mail de teste para validar as configurações SMTP do <strong>Zenith-Go</strong>.</p>
		
		<dl class="details-grid">
			<dt>Destinatário:</dt>
			<dd>%s</dd>
			<dt>Data e Hora:</dt>
			<dd>%s</dd>
			<dt>Status:</dt>
			<dd style="background-color: #e6fffa; color: #00473e; border: 1px solid #b2f5ea;"><strong>OK</strong></dd>
		</dl>

		<div class="code-block">
			<h3>Mensagem do Sistema:</h3>
			<pre>SMTP Connected successfully.
TLS Handshake verified (InsecureSkipVerify=true).
Authentication accepted.</pre>
		</div>
	`, recipient, time.Now().Format("02/01/2006, 15:04:05"))

	fullHtml := s.getHtmlTemplate(title, bodyContent)

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
	message += "\r\n" + fullHtml

	slog.Info("Enviando e-mail de teste...", "to", recipient)
	return s.sendMail([]string{recipient}, []byte(message))
}

// SendError envia alerta de erro formatado
func (s *EmailService) SendError(err error, contextInfo map[string]string) {
	if !s.cfg.EmailEnabled || len(s.cfg.EmailRecipients) == 0 {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Panic recuperado no envio de e-mail", "recover", r)
			}
		}()

		title := "Alerta de Erro no Sistema"
		subject := fmt.Sprintf("🚨 [Zenith-Go] Erro: %s", truncate(err.Error(), 50))

		// Monta a Grid de Detalhes dinamicamente
		detailsHtml := ""
		
		keys := make([]string, 0, len(contextInfo))
		for k := range contextInfo {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		detailsHtml += fmt.Sprintf("<dt>Data e Hora:</dt><dd>%s</dd>", time.Now().Format("02/01/2006, 15:04:05"))
		
		for _, k := range keys {
			val := contextInfo[k]
			detailsHtml += fmt.Sprintf("<dt>%s:</dt><dd>%s</dd>", k, val)
		}

		bodyContent := fmt.Sprintf(`
			<h2>%s</h2>
			<dl class="details-grid">
				%s
			</dl>

			<div class="code-block">
				<h3>Detalhes do Erro:</h3>
				<pre>%s</pre>
			</div>
		`, truncate(err.Error(), 100), detailsHtml, err.Error())

		fullHtml := s.getHtmlTemplate(title, bodyContent)

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
		message += "\r\n" + fullHtml

		if sendErr := s.sendMail(s.cfg.EmailRecipients, []byte(message)); sendErr != nil {
			slog.Error("Falha ao enviar e-mail de notificação", "error", sendErr)
		} else {
			slog.Info("E-mail de notificação enviado com sucesso")
		}
	}()
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}