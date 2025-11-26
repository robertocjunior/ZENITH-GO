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

// Authenticate realiza o login do SISTEMA (Service Account) com Retry
func (c *Client) Authenticate(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	slog.Info("Autenticando Sistema (Service Account) no Sankhya...")

	url := fmt.Sprintf("%s/login", c.cfg.ApiUrl)
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(2 * time.Second)
			slog.Info("Retentando login no Sankhya devido a erro anterior...", "tentativa", i+1)
		}

		// Recria o request a cada tentativa (pois o Body é consumido)
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
			lastErr = fmt.Errorf("falha na rede durante login: %w", err)
			slog.Warn("Erro de rede no login", "error", err)
			continue
		}

		// Lê o corpo para diagnóstico
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("login falhou com status %d: %s", resp.StatusCode, string(bodyBytes))

			// Se for erro 5xx (servidor), tenta novamente
			if resp.StatusCode >= 500 {
				slog.Warn("Erro interno do Sankhya (500), retentando...", "status", resp.StatusCode)
				continue
			}

			// Se for 4xx (erro de cliente/credenciais), para imediatamente
			slog.Error("Erro fatal de login (não será retentado)", "status", resp.StatusCode, "response", string(bodyBytes))
			return lastErr
		}

		// Sucesso: Decodifica
		var result loginResponse
		if err := json.NewDecoder(bytes.NewBuffer(bodyBytes)).Decode(&result); err != nil {
			lastErr = fmt.Errorf("erro ao decodificar resposta: %w", err)
			continue
		}

		if result.BearerToken == "" {
			jsonErr, _ := json.Marshal(result.Error)
			lastErr = fmt.Errorf("token não retornado pelo ERP: %s", string(jsonErr))
			slog.Warn("Token vazio na resposta", "response", string(jsonErr))
			continue
		}

		c.bearerToken = result.BearerToken
		// Aumentado para 20 minutos para evitar logins frequentes (Sankhya geralmente dura 30m)
		c.tokenExpiry = time.Now().Add(20 * time.Minute)

		slog.Info("Autenticação do sistema renovada com sucesso", "expiry", c.tokenExpiry)
		return nil
	}

	slog.Error("Todas as tentativas de login falharam", "last_error", lastErr)
	return lastErr
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

	// Se expirou, renova
	slog.Info("Token de sistema expirado ou inexistente. Renovando...")
	if err := c.Authenticate(ctx); err != nil {
		return "", err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bearerToken, nil
}

// executeQuery é o método base para rodar qualquer SQL no Sankhya
func (c *Client) executeQuery(ctx context.Context, sql string) ([][]any, error) {
	sysToken, err := c.GetToken(ctx)
	if err != nil {
		return nil, err
	}

	reqBody := dbExplorerRequest{
		ServiceName: "DbExplorerSP.executeQuery",
	}
	reqBody.RequestBody.SQL = sql
	reqBody.RequestBody.Params = make(map[string]any)

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

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
		slog.Error("Erro HTTP na query", "status", resp.StatusCode, "body", string(bodyBytes))
		return nil, fmt.Errorf("erro HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result dbExplorerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erro decodificando query: %w", err)
	}

	if result.Status != "1" {
		slog.Error("Erro na execução de SQL (Sankhya)", "status", result.Status)
		return nil, fmt.Errorf("erro no DbExplorerSP status: %s", result.Status)
	}

	return result.ResponseBody.Rows, nil
}

func sanitizeStringForSql(s string) string {
	return strings.ReplaceAll(s, "'", "")
}