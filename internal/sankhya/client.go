package sankhya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("token", c.cfg.Token)
	req.Header.Set("appkey", c.cfg.AppKey)
	req.Header.Set("username", c.cfg.Username)
	req.Header.Set("password", c.cfg.Password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("Falha na requisição de login do sistema", "error", err)
		return fmt.Errorf("falha na requisição de login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Login do sistema falhou", "status", resp.StatusCode)
		return fmt.Errorf("login falhou com status: %d", resp.StatusCode)
	}

	var result loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("erro ao decodificar resposta: %w", err)
	}

	if result.BearerToken == "" {
		return fmt.Errorf("token não retornado pelo ERP")
	}

	c.bearerToken = result.BearerToken
	c.tokenExpiry = time.Now().Add(4*time.Minute + 50*time.Second)

	slog.Info("Autenticação do sistema renovada com sucesso", "expiry", c.tokenExpiry)
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

	// slog.Debug("Executando SQL", "sql_prefix", sql[:min(len(sql), 50)]+"...") // Debug de SQL (opcional, pode poluir)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result dbExplorerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erro decodificando query: %w", err)
	}

	if result.Status != "1" {
		slog.Error("Erro na execução de SQL", "status", result.Status)
		return nil, fmt.Errorf("erro no DbExplorerSP status: %s", result.Status)
	}

	return result.ResponseBody.Rows, nil
}

func sanitizeStringForSql(s string) string {
	return strings.ReplaceAll(s, "'", "")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}