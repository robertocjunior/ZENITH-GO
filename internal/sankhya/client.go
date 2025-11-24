package sankhya

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"zenith-go/internal/config"
)

// --- Definição de Erros Públicos (Exportados) ---
var (
	ErrUserNotFound      = errors.New("usuário inexistente ou nome incorreto")
	ErrUserNotAuthorized = errors.New("usuário não possui autorização de acesso (AD_APPPERM)")
)

// --- Structs de Login do Sistema ---
type loginResponse struct {
	BearerToken string `json:"bearerToken"`
	Error       any    `json:"error"`
}

// --- Structs de Login Mobile ---
type simpleValue struct {
	Value string `json:"$"`
}

type mobileLoginRequest struct {
	ServiceName string `json:"serviceName"`
	RequestBody struct {
		NomUsu        simpleValue `json:"NOMUSU"`
		Interno       simpleValue `json:"INTERNO"`
		KeepConnected simpleValue `json:"KEEPCONNECTED"`
	} `json:"requestBody"`
}

type mobileLoginResponse struct {
	Status       string `json:"status"`
	ResponseBody struct {
		JSessionID simpleValue `json:"jsessionid"`
	} `json:"responseBody"`
}

// --- Structs DbExplorerSP (Consulta SQL) ---
type dbExplorerRequest struct {
	ServiceName string `json:"serviceName"`
	RequestBody struct {
		SQL    string `json:"sql"`
		Params map[string]any `json:"params"`
	} `json:"requestBody"`
}

type dbExplorerResponse struct {
	Status       string `json:"status"`
	ResponseBody struct {
		Rows [][]any `json:"rows"`
	} `json:"responseBody"`
}

// Client gerencia a comunicação com o ERP
type Client struct {
	cfg         *config.Config
	httpClient  *http.Client
	bearerToken string
	tokenExpiry time.Time
	mu          sync.RWMutex
}

// NewClient cria uma nova instância do cliente
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Authenticate realiza o login do SISTEMA (AppKey/Token)
func (c *Client) Authenticate() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	url := fmt.Sprintf("%s/login", c.cfg.ApiUrl)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

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

	// Armazena o token e define expiração para 4m50s
	c.bearerToken = result.BearerToken
	c.tokenExpiry = time.Now().Add(4*time.Minute + 50*time.Second)

	fmt.Println(">>> Autenticação no Sankhya realizada com sucesso!")
	return nil
}

// GetToken retorna o token do sistema válido.
func (c *Client) GetToken() (string, error) {
	c.mu.RLock()
	if c.bearerToken != "" && time.Now().Before(c.tokenExpiry) {
		token := c.bearerToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	if err := c.Authenticate(); err != nil {
		return "", err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bearerToken, nil
}

// VerifyUserAccess verifica existência e permissão em UMA ÚNICA CONSULTA
func (c *Client) VerifyUserAccess(username string) (float64, error) {
	sysToken, err := c.GetToken()
	if err != nil {
		return 0, err
	}

	// SQL OTIMIZADO:
	// 1. Busca CODUSU
	// 2. Cria coluna 'PERMITIDO' (TRUE/FALSE) verificando AD_APPPERM
	sqlQuery := fmt.Sprintf(`
		SELECT 
			U.CODUSU, 
			CASE 
				WHEN EXISTS (SELECT 1 FROM AD_APPPERM P WHERE P.CODUSU = U.CODUSU) THEN 'TRUE' 
				ELSE 'FALSE' 
			END AS PERMITIDO 
		FROM TSIUSU U 
		WHERE U.NOMEUSU = '%s'`, strings.ToUpper(username))

	reqBody := dbExplorerRequest{
		ServiceName: "DbExplorerSP.executeQuery",
	}
	reqBody.RequestBody.SQL = sqlQuery
	reqBody.RequestBody.Params = make(map[string]any)

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return 0, err
	}

	url := fmt.Sprintf("%s/gateway/v1/mge/service.sbr?serviceName=DbExplorerSP.executeQuery&outputType=json", c.cfg.ApiUrl)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+sysToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result dbExplorerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("erro ao decodificar dbexplorer: %w", err)
	}

	if result.Status != "1" {
		return 0, fmt.Errorf("erro na consulta SQL (Status %s)", result.Status)
	}

	// --- Lógica de Diferenciação de Erros ---

	// CASO 1: Usuário Inexistente (Nenhuma linha retornada)
	if len(result.ResponseBody.Rows) == 0 {
		return 0, ErrUserNotFound
	}

	// Sankhya retorna números como float64 na interface genérica
	row := result.ResponseBody.Rows[0]
	codUsu := row[0].(float64)
	permitido := row[1].(string)

	// CASO 2: Usuário Existe, mas sem Permissão (Coluna PERMITIDO == FALSE)
	if permitido != "TRUE" {
		return 0, ErrUserNotAuthorized
	}

	// CASO 3: Sucesso
	return codUsu, nil
}

// LoginUser realiza o login do usuário final na API do Sankhya
func (c *Client) LoginUser(username, password string) (string, error) {
	sysToken, err := c.GetToken()
	if err != nil {
		return "", fmt.Errorf("erro de autenticação do sistema: %w", err)
	}

	reqBody := mobileLoginRequest{
		ServiceName: "MobileLoginSP.login",
	}
	reqBody.RequestBody.NomUsu.Value = username
	reqBody.RequestBody.Interno.Value = password
	reqBody.RequestBody.KeepConnected.Value = "S"

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/gateway/v1/mge/service.sbr?serviceName=MobileLoginSP.login&outputType=json", c.cfg.ApiUrl)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+sysToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("erro na requisição sankhya: status %d", resp.StatusCode)
	}

	var result mobileLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Status != "1" {
		return "", fmt.Errorf("usuário ou senha inválidos (ERP recusou login)")
	}

	return result.ResponseBody.JSessionID.Value, nil
}