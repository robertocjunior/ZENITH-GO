package notification

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/smtp"
	"regexp"
	"sort"
	"strings"
	"time"
	"zenith-go/internal/config"
)

// Link direto para a logo (Raw GitHub)
const zenithLogoUrl = "https://raw.githubusercontent.com/robertocjunior/assets/refs/heads/main/zenith.svg"

type EmailService struct {
	cfg *config.Config
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{cfg: cfg}
}

// Regex para limpar tags HTML de strings simples (assuntos/títulos)
var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

// Remove tags HTML para usar no Assunto do Email
func stripTags(s string) string {
	return htmlTagRegex.ReplaceAllString(s, "")
}

// isHTML detecta se a string parece conter HTML do Sankhya
func isHTML(s string) bool {
	sLower := strings.ToLower(s)
	return strings.Contains(sLower, "<br") || 
		   strings.Contains(sLower, "<b") || 
		   strings.Contains(sLower, "<p") || 
		   strings.Contains(sLower, "<font") ||
		   strings.Contains(sLower, "<html")
}

// cleanErrorMessage remove poluição do HTML legado do Sankhya
func cleanErrorMessage(rawMsg string) string {
	// 1. Remove escapes de aspas duplas vindos do JSON/Oracle (ex: \" -> ")
	msg := strings.ReplaceAll(rawMsg, `\"`, `"`)

	// 2. REMOVE A IMAGEM ESPECÍFICA SOLICITADA
	msg = strings.ReplaceAll(msg, `<img src="http://www.sankhya.com.br/imagens/logo-sankhya.png"></img>`, "")
	// Caso venha com escape
	msg = strings.ReplaceAll(msg, `<img src=\"http://www.sankhya.com.br/imagens/logo-sankhya.png\"></img>`, "")
	// Caso venha self-closing
	msg = strings.ReplaceAll(msg, `<img src="http://www.sankhya.com.br/imagens/logo-sankhya.png"/>`, "")

	// 3. Remove links envolvendo a imagem se houver wrapper
	msg = strings.ReplaceAll(msg, `<a href="http://www.sankhya.com.br" target="_blank"></a>`, "")

	// 4. Reduz múltiplos <br> consecutivos para apenas dois
	for strings.Contains(msg, "<br><br><br>") {
		msg = strings.ReplaceAll(msg, "<br><br><br>", "<br><br>")
	}
	
	// 5. Remove excesso de quebras no início ou tags vazias comuns
	msg = strings.ReplaceAll(msg, "<p align='center'></p>", "")

	return msg
}

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
        .code-block { margin-top: 30px; }
        .code-block h3 { font-size: 16px; color: #333; margin-bottom: 10px; border-left: 4px solid #00529B; padding-left: 10px; }
        
        /* Estilo para erros em texto puro */
        .code-block pre { background-color: #2d2d2d; color: #f2f2f2; padding: 15px; border-radius: 6px; white-space: pre-wrap; word-wrap: break-word; font-family: 'Courier New', Courier, monospace; font-size: 13px; }
        
        /* Estilo MELHORADO para erros em HTML Sankhya */
        .html-error-container { 
            background-color: #ffffff; 
            border: 1px solid #e0e0e0; 
            padding: 20px; 
            border-radius: 8px; 
            overflow-y: auto; 
            max-height: 500px; 
            box-shadow: inset 0 0 10px rgba(0,0,0,0.03);
        }

        /* RESET AGRESSIVO: Força que todo conteúdo dentro do erro (incluindo tags <font> do Sankhya)
           tenha tamanho e cor normais, ignorando o size="12" gigante.
        */
        .html-error-container * {
            font-size: 14px !important;
            font-family: Arial, sans-serif !important;
            color: #333 !important;
            line-height: 1.5 !important;
            background-color: transparent !important;
        }

        /* Mantém negrito onde deve ter */
        .html-error-container b, .html-error-container strong {
            font-weight: bold !important;
        }

        .footer { background-color: #eef2f5; color: #888; font-size: 12px; text-align: center; padding: 20px; }
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

func (s *EmailService) SendTestEmail(recipient string) error {
	if !s.cfg.EmailEnabled {
		return fmt.Errorf("envio de e-mail desabilitado no .env")
	}

	title := "Teste de Configuração"
	subject := "🧪 [Zenith-Go] Teste de Envio de E-mail"

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

		// 1. Limpeza preliminar para o título
		rawErrString := err.Error()
		cleanErrTitle := stripTags(rawErrString)
		
		title := "Alerta de Erro no Sistema"
		subject := fmt.Sprintf("🚨 [Zenith-Go] Erro: %s", truncate(cleanErrTitle, 50))

		// 2. Extrai Payload e remove do map
		payloadBlock := ""
		if payload, ok := contextInfo["Payload"]; ok {
			payloadBlock = fmt.Sprintf(`<div class="code-block"><h3>Payload da Requisição (Sanitizado):</h3><pre>%s</pre></div>`, payload)
			delete(contextInfo, "Payload")
		}

		// 3. Monta a Grid de Detalhes
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

		// 4. Lógica de renderização do erro principal
		errorBlock := ""
		
		// Limpa o HTML da mensagem de erro (apenas remove imagens e limpa estrutura, SEM mexer no encoding)
		finalErrorMsg := cleanErrorMessage(rawErrString)

		if isHTML(finalErrorMsg) {
			// Se for HTML (agora limpo), renderiza no container com CSS resetado
			errorBlock = fmt.Sprintf(`
			<div class="code-block">
				<h3>Detalhes do Erro (Formatado):</h3>
				<div class="html-error-container">
					%s
				</div>
			</div>`, finalErrorMsg)
		} else {
			// Texto puro
			errorBlock = fmt.Sprintf(`
			<div class="code-block">
				<h3>Stack Trace / Detalhes do Erro:</h3>
				<pre>%s</pre>
			</div>`, finalErrorMsg)
		}

		bodyContent := fmt.Sprintf(`
			<h2>%s</h2>
			<dl class="details-grid">
				%s
			</dl>
			
			%s

			%s
		`, truncate(cleanErrTitle, 100), detailsHtml, payloadBlock, errorBlock)

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