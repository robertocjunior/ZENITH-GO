package sankhya

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"zenith-go/internal/config"
)

// --- Erros Públicos ---
var (
	ErrUserNotFound          = errors.New("usuário inexistente ou nome incorreto")
	ErrUserNotAuthorized     = errors.New("usuário não possui autorização de acesso (AD_APPPERM)")
	ErrDevicePendingApproval = errors.New("dispositivo não autorizado. Solicite a liberação ao administrador")
)

// --- Structs de Login ---
type loginResponse struct {
	BearerToken string `json:"bearerToken"`
	Error       any    `json:"error"`
}

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

// --- Structs DatasetSP.save (Inserção) ---
type datasetSaveRequest struct {
	ServiceName string `json:"serviceName"`
	RequestBody struct {
		EntityName string `json:"entityName"`
		Fields     []string `json:"fields"`
		Records    []datasetRecord `json:"records"`
	} `json:"requestBody"`
}

type datasetRecord struct {
	Values map[string]string `json:"values"`
}

type datasetSaveResponse struct {
	Status string `json:"status"`
}

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

// Authenticate login do SISTEMA
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

	c.bearerToken = result.BearerToken
	c.tokenExpiry = time.Now().Add(4*time.Minute + 50*time.Second)

	fmt.Println(">>> Autenticação no Sankhya realizada com sucesso!")
	return nil
}

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

// VerifyUserAccess valida permissão básica do usuário
func (c *Client) VerifyUserAccess(username string) (float64, error) {
	sysToken, err := c.GetToken()
	if err != nil {
		return 0, err
	}

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
		return 0, fmt.Errorf("erro no json dbexplorer: %w", err)
	}

	if len(result.ResponseBody.Rows) == 0 {
		return 0, ErrUserNotFound
	}

	row := result.ResponseBody.Rows[0]
	codUsu := row[0].(float64)
	permitido := row[1].(string)

	if permitido != "TRUE" {
		return 0, ErrUserNotAuthorized
	}

	return codUsu, nil
}

// VerifyDevice verifica se o device está autorizado ou registra se não existir
func (c *Client) VerifyDevice(codUsu int, deviceToken string) error {
	sysToken, err := c.GetToken()
	if err != nil {
		return err
	}

	// 1. Consulta SQL para verificar o device
	sqlQuery := fmt.Sprintf(`
		SELECT DEVICETOKEN, CODUSU, ATIVO 
		FROM AD_DISPAUT 
		WHERE CODUSU = %d AND DEVICETOKEN = '%s'`, codUsu, deviceToken)

	reqBody := dbExplorerRequest{
		ServiceName: "DbExplorerSP.executeQuery",
	}
	reqBody.RequestBody.SQL = sqlQuery
	reqBody.RequestBody.Params = make(map[string]any)

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/gateway/v1/mge/service.sbr?serviceName=DbExplorerSP.executeQuery&outputType=json", c.cfg.ApiUrl)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+sysToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result dbExplorerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("erro json device check: %w", err)
	}

	// CENÁRIO 1: Device NÃO existe no banco -> Inserir e retornar erro
	if len(result.ResponseBody.Rows) == 0 {
		if regErr := c.registerDevice(sysToken, codUsu, deviceToken); regErr != nil {
			return fmt.Errorf("erro ao registrar novo device: %w", regErr)
		}
		return ErrDevicePendingApproval
	}

	// CENÁRIO 2: Device existe -> Verificar se está ATIVO
	row := result.ResponseBody.Rows[0]
	ativo := row[2].(string) // Terceira coluna (index 2) é ATIVO

	if ativo == "S" {
		return nil // Sucesso, autorizado
	}

	// Se ATIVO = 'N'
	return ErrDevicePendingApproval
}

// registerDevice insere o novo dispositivo na tabela AD_DISPAUT
func (c *Client) registerDevice(token string, codUsu int, deviceToken string) error {
	// Data atual DD/MM/YYYY
	dhGer := time.Now().Format("02/01/2006")

	reqBody := datasetSaveRequest{
		ServiceName: "DatasetSP.save",
	}
	reqBody.RequestBody.EntityName = "AD_DISPAUT"
	reqBody.RequestBody.Fields = []string{"CODUSU", "DEVICETOKEN", "DESCRDISP", "ATIVO", "DHGER"}
	
	// Monta o registro
	record := datasetRecord{
		Values: map[string]string{
			"0": strconv.Itoa(codUsu), // CODUSU
			"1": deviceToken,          // DEVICETOKEN
			"2": "Novo Dispositivo",   // DESCRDISP (Padrão)
			"3": "N",                  // ATIVO (Padrão N)
			"4": dhGer,                // DHGER
		},
	}
	reqBody.RequestBody.Records = []datasetRecord{record}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/gateway/v1/mge/service.sbr?serviceName=DatasetSP.save&outputType=json", c.cfg.ApiUrl)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result datasetSaveResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("erro decode save device: %w", err)
	}

	if result.Status != "1" {
		return fmt.Errorf("erro ao salvar device no ERP (status %s)", result.Status)
	}

	return nil
}

// LoginUser realiza login final
func (c *Client) LoginUser(username, password string) (string, error) {
	sysToken, err := c.GetToken()
	if err != nil {
		return "", fmt.Errorf("erro sistema: %w", err)
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
		return "", fmt.Errorf("status http erro: %d", resp.StatusCode)
	}

	var result mobileLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Status != "1" {
		return "", fmt.Errorf("credenciais inválidas")
	}

	return result.ResponseBody.JSessionID.Value, nil
}