package sankhya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"zenith-go/internal/config"
)

// Client estrutura principal
type Client struct {
	cfg         *config.Config
	httpClient  *http.Client
	bearerToken string
	tokenExpiry time.Time
	mu          sync.RWMutex
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Authenticate realiza o login do SISTEMA (Service Account)
func (c *Client) Authenticate(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	slog.Info("Autenticando Sistema (Service Account) no Sankhya...")

	url := fmt.Sprintf("%s/login", c.cfg.ApiUrl)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return err
	}

	req.Header.Set("token", c.cfg.Token)
	req.Header.Set("appkey", c.cfg.AppKey)
	req.Header.Set("username", c.cfg.Username)
	req.Header.Set("password", c.cfg.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("Falha na requisição de login do sistema", "error", err)
		return fmt.Errorf("falha na requisição de login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := string(bodyBytes)
		
		slog.Error("Login do sistema falhou", 
			"status", resp.StatusCode, 
			"response", errorMsg,
			"url", url,
		)
		return fmt.Errorf("login falhou com status %d: %s", resp.StatusCode, errorMsg)
	}

	var result loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("erro ao decodificar resposta: %w", err)
	}

	if result.BearerToken == "" {
		jsonErr, _ := json.Marshal(result.Error)
		slog.Error("Token não retornado", "sankhya_error", string(jsonErr))
		return fmt.Errorf("token não retornado pelo ERP: %s", string(jsonErr))
	}

	c.bearerToken = result.BearerToken
	
	// ALTERAÇÃO AQUI: Usa o tempo configurado no .env (ou padrão 5 min)
	expiryMinutes := time.Duration(c.cfg.SankhyaTokenExpiryMinutes)
	c.tokenExpiry = time.Now().Add(expiryMinutes * time.Minute)

	slog.Info("Autenticação do sistema renovada com sucesso", "expiry", c.tokenExpiry, "minutes_valid", expiryMinutes)
	return nil
}

// GetToken gerencia o token Bearer, renovando se necessário
func (c *Client) GetToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.bearerToken != "" && time.Now().Before(c.tokenExpiry) {
		token := c.bearerToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	slog.Info("Token de sistema expirado (localmente) ou inexistente. Renovando...")
	if err := c.Authenticate(ctx); err != nil {
		return "", err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bearerToken, nil
}

// executeQuery com RETRY AUTOMÁTICO em caso de erro de sessão (Status 3)
func (c *Client) executeQuery(ctx context.Context, sql string) ([][]any, error) {
	maxAttempts := 2
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		sysToken, err := c.GetToken(ctx)
		if err != nil {
			return nil, err
		}

		reqBody := dbExplorerRequest{ServiceName: "DbExplorerSP.executeQuery"}
		reqBody.RequestBody.SQL = sql
		reqBody.RequestBody.Params = make(map[string]any)
		jsonData, _ := json.Marshal(reqBody)

		url := fmt.Sprintf("%s/gateway/v1/mge/service.sbr?serviceName=DbExplorerSP.executeQuery&outputType=json", c.cfg.ApiUrl)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+sysToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("erro HTTP %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var result dbExplorerResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("erro decodificando query: %w", err)
		}

		if result.Status == "1" {
			return result.ResponseBody.Rows, nil
		}

		if result.Status == "3" {
			slog.Warn("Sessão Sankhya expirada (Status 3) durante query. Tentativa de renovação...", "attempt", attempt)
			
			c.mu.Lock()
			c.tokenExpiry = time.Time{} // Zera validade para forçar re-login
			c.mu.Unlock()
			
			if attempt == maxAttempts {
				lastErr = fmt.Errorf("erro de sessão persistente após retry (Status 3)")
			}
			continue 
		}

		slog.Error("Erro na execução de SQL (Sankhya)", "status", result.Status)
		return nil, fmt.Errorf("erro no DbExplorerSP status: %s", result.Status)
	}

	return nil, lastErr
}

func (c *Client) KeepAlive(ctx context.Context, snkSessionID string) error {
	// Constroi a URL
	baseURL := strings.TrimRight(c.cfg.SankhyaRenewUrl, "/")
	url := fmt.Sprintf("%s/placemm/place/status?action=list&ignoreUpdSessionTime=true", baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Cookie", fmt.Sprintf("JSESSIONID=%s", snkSessionID))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("erro de rede no keepalive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status http inválido: %d", resp.StatusCode)
	}

	// Estrutura simples para validar a resposta
	var result struct {
		Success bool `json:"success"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("erro ao decodificar json: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("sankhya retornou success: false")
	}

	return nil
}

func sanitizeStringForSql(s string) string {
	return strings.ReplaceAll(s, "'", "")
}