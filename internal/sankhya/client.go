package sankhya

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
	"zenith-go/internal/config"
)

// Client gerencia a comunicação com o ERP
type Client struct {
	cfg         *config.Config
	httpClient  *http.Client
	bearerToken string
	tokenExpiry time.Time
	mu          sync.RWMutex // Mutex para segurança em concorrência (thread-safe)
}

// Struct para a resposta do login
type loginResponse struct {
	BearerToken string `json:"bearerToken"`
	Error       any    `json:"error"`
}

// NewClient cria uma nova instância do cliente
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Authenticate realiza o login e armazena o token
func (c *Client) Authenticate() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	url := fmt.Sprintf("%s/login", c.cfg.ApiUrl)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	// Define os headers conforme solicitado
	req.Header.Set("token", c.cfg.Token)
	req.Header.Set("appkey", c.cfg.AppKey)
	req.Header.Set("username", c.cfg.Username)
	req.Header.Set("password", c.cfg.Password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("falha na requisição de login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login falhou com status: %d", resp.StatusCode)
	}

	var result loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("erro ao decodificar resposta: %w", err)
	}

	if result.BearerToken == "" {
		return fmt.Errorf("token não retornado pelo ERP")
	}

	// Armazena o token e define expiração para 4m50s (margem de segurança de 10s)
	c.bearerToken = result.BearerToken
	c.tokenExpiry = time.Now().Add(4*time.Minute + 50*time.Second)

	fmt.Println(">>> Autenticação no Sankhya realizada com sucesso!")
	return nil
}

// GetToken retorna o token válido. Se expirado, tenta renovar.
func (c *Client) GetToken() (string, error) {
	c.mu.RLock()
	// Verifica se o token existe e ainda é válido
	if c.bearerToken != "" && time.Now().Before(c.tokenExpiry) {
		token := c.bearerToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	// Se expirou ou não existe, renova
	if err := c.Authenticate(); err != nil {
		return "", err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bearerToken, nil
}