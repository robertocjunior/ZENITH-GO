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

	// Verifica novamente se já foi renovado por outra thread
	if c.bearerToken != "" && time.Now().Before(c.tokenExpiry) {
		return nil
	}

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
	
	expiryMinutes := time.Duration(c.cfg.SankhyaTokenExpiryMinutes)
	c.tokenExpiry = time.Now().Add(expiryMinutes * time.Minute)

	slog.Info("Autenticação do sistema renovada com sucesso", "expiry", c.tokenExpiry, "minutes_valid", expiryMinutes)
	return nil
}

// GetToken gerencia o token Bearer, renovando se necessário
func (c *Client) GetToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.bearerToken != "" && time.Now().Add(30*time.Second).Before(c.tokenExpiry) {
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

// KeepAlive realiza a chamada para manter a sessão ativa
func (c *Client) KeepAlive(ctx context.Context, snkSessionID string) error {
	baseURL := strings.TrimRight(c.cfg.SankhyaRenewUrl, "/")
	url := fmt.Sprintf("%s/placemm/place/status?action=list&ignoreUpdSessionTime=true", baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Cookie", fmt.Sprintf("JSESSIONID=%s", snkSessionID))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "keep-alive")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("erro de rede no keepalive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status http inválido: %d", resp.StatusCode)
	}

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

// executeQuery com RETRY AUTOMÁTICO em caso de erro de sessão (Status 3 ou 0)
func (c *Client) executeQuery(ctx context.Context, sql string) ([][]any, error) {
	maxAttempts := 3
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

		// CORREÇÃO AQUI: URL Fixa sem placeholder sobrando
		url := fmt.Sprintf("%s/gateway/v1/mge/service.sbr?serviceName=DbExplorerSP.executeQuery&outputType=json", c.cfg.ApiUrl)
		
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+sysToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			slog.Warn("Erro de rede ao conectar no Sankhya. Tentando novamente...", "attempt", attempt, "error", err)
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
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

		// TRATAMENTO DE ERROS PARA RETRY
		if result.Status == "3" || result.Status == "0" {
			slog.Warn("Sessão Sankhya instável (Status "+result.Status+"). Renovando token...", "attempt", attempt)
			
			c.mu.Lock()
			c.tokenExpiry = time.Time{} 
			c.mu.Unlock()
			
			if attempt == maxAttempts {
				lastErr = fmt.Errorf("erro persistente no Sankhya (Status %s): %s", result.Status, result.StatusMessage)
			}
			time.Sleep(1 * time.Second)
			continue 
		}

		slog.Error("Erro na execução de SQL (Sankhya)", "status", result.Status, "msg", result.StatusMessage)
		return nil, fmt.Errorf("erro no DbExplorerSP status: %s - %s", result.Status, result.StatusMessage)
	}

	return nil, lastErr
}

// ExecuteServiceAsSystem com RETRY AUTOMÁTICO
func (c *Client) ExecuteServiceAsSystem(ctx context.Context, serviceName string, requestBody any) (*TransactionResponse, error) {
	maxAttempts := 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		sysToken, err := c.GetToken(ctx)
		if err != nil {
			return nil, err
		}

		url := fmt.Sprintf("%s/gateway/v1/mge/service.sbr?serviceName=%s&outputType=json", c.cfg.ApiUrl, serviceName)
		payload := ServiceRequest{ServiceName: serviceName, RequestBody: requestBody}
		jsonData, _ := json.Marshal(payload)

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+sysToken)
		req.Header.Set("Content-Type", "application/json")

		slog.Debug("Calling Sankhya System Service", "service", serviceName, "attempt", attempt)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			slog.Warn("Erro de rede no serviço. Tentando novamente...", "service", serviceName, "error", err)
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		defer resp.Body.Close()

		var result TransactionResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("erro ao decodificar resposta: %w", err)
		}

		if result.Status == "1" {
			return &result, nil
		}

		// Tratamento de Sessão Expirada ou Instável
		isTokenError := result.Status == "3" || result.Status == "0" ||
						(result.Status == "0" && (strings.Contains(result.StatusMessage, "Token") || strings.Contains(result.StatusMessage, "Sessão")))

		if isTokenError {
			slog.Warn("Token instável ou rejeitado. Renovando...", "service", serviceName, "status", result.Status)
			c.mu.Lock()
			c.tokenExpiry = time.Time{} 
			c.mu.Unlock()
			
			if attempt == maxAttempts {
				lastErr = fmt.Errorf("erro persistente em %s: %s", serviceName, result.StatusMessage)
			}
			time.Sleep(1 * time.Second)
			continue
		}

		slog.Error("Sankhya System API Error", "service", serviceName, "status", result.Status, "msg", result.StatusMessage)
		return nil, fmt.Errorf("erro na System API (%s): %s", serviceName, result.StatusMessage)
	}

	return nil, lastErr
}

func sanitizeStringForSql(s string) string {
	return strings.ReplaceAll(s, "'", "")
}