package sankhya

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
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

// --- Structs de Permissões ---
type UserPermissions struct {
	CodUsu       int    `json:"CODUSU"`
	ListaCodigos string `json:"LISTA_CODIGOS"`
	ListaNomes   string `json:"LISTA_NOMES"`
	Transf       bool   `json:"TRANSF"`
	Baixa        bool   `json:"BAIXA"`
	Pick         bool   `json:"PICK"`
	Corre        bool   `json:"CORRE"`
	BxaPick      bool   `json:"BXAPICK"`
	CriaPick     bool   `json:"CRIAPICK"`
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

// executeQuery é um helper privado para rodar SQLs
func (c *Client) executeQuery(sql string) ([][]any, error) {
	sysToken, err := c.GetToken()
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
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
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

	var result dbExplorerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erro decodificando query: %w", err)
	}

	return result.ResponseBody.Rows, nil
}

// VerifyUserAccess valida permissão básica do usuário
func (c *Client) VerifyUserAccess(username string) (float64, error) {
	sqlQuery := fmt.Sprintf(`
		SELECT 
			U.CODUSU, 
			CASE 
				WHEN EXISTS (SELECT 1 FROM AD_APPPERM P WHERE P.CODUSU = U.CODUSU) THEN 'TRUE' 
				ELSE 'FALSE' 
			END AS PERMITIDO 
		FROM TSIUSU U 
		WHERE U.NOMEUSU = '%s'`, strings.ToUpper(username))

	rows, err := c.executeQuery(sqlQuery)
	if err != nil {
		return 0, err
	}

	if len(rows) == 0 {
		return 0, ErrUserNotFound
	}

	row := rows[0]
	codUsu := row[0].(float64)
	permitido := row[1].(string)

	if permitido != "TRUE" {
		return 0, ErrUserNotAuthorized
	}

	return codUsu, nil
}

// VerifyDevice verifica se o device está autorizado
func (c *Client) VerifyDevice(codUsu int, deviceToken string) error {
	sqlQuery := fmt.Sprintf(`
		SELECT DEVICETOKEN, CODUSU, ATIVO 
		FROM AD_DISPAUT 
		WHERE CODUSU = %d AND DEVICETOKEN = '%s'`, codUsu, deviceToken)

	rows, err := c.executeQuery(sqlQuery)
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		sysToken, _ := c.GetToken()
		if regErr := c.registerDevice(sysToken, codUsu, deviceToken); regErr != nil {
			return fmt.Errorf("erro ao registrar novo device: %w", regErr)
		}
		return ErrDevicePendingApproval
	}

	ativo := rows[0][2].(string)
	if ativo == "S" {
		return nil
	}
	return ErrDevicePendingApproval
}

func (c *Client) registerDevice(token string, codUsu int, deviceToken string) error {
	dhGer := time.Now().Format("02/01/2006")
	reqBody := datasetSaveRequest{
		ServiceName: "DatasetSP.save",
	}
	reqBody.RequestBody.EntityName = "AD_DISPAUT"
	reqBody.RequestBody.Fields = []string{"CODUSU", "DEVICETOKEN", "DESCRDISP", "ATIVO", "DHGER"}

	record := datasetRecord{
		Values: map[string]string{
			"0": strconv.Itoa(codUsu),
			"1": deviceToken,
			"2": "Novo Dispositivo",
			"3": "N",
			"4": dhGer,
		},
	}
	reqBody.RequestBody.Records = []datasetRecord{record}
	jsonData, _ := json.Marshal(reqBody)

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
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Status != "1" {
		return fmt.Errorf("status erro ao salvar device")
	}
	return nil
}

func (c *Client) GetUserPermissions(codUsu int) (*UserPermissions, error) {
	sqlQuery := fmt.Sprintf(`
		SELECT 
			LISTAGG(d.CODARM, ', ') WITHIN GROUP (ORDER BY d.CODARM) AS LISTA_CODIGOS, 
			LISTAGG(d.CODARM || ' - ' || a.DESARM, ', ') WITHIN GROUP (ORDER BY d.CODARM) AS LISTA_NOMES, 
			p.CODUSU, p.TRANSF, p.BAIXA, p.PICK, p.CORRE, p.BXAPICK, p.CRIAPICK 
		FROM AD_APPPERM p 
		JOIN AD_PERMEND d ON d.NUMREG = p.NUMREG 
		JOIN AD_CADARM a ON a.CODARM = d.CODARM 
		WHERE p.CODUSU = %d 
		GROUP BY p.CODUSU, p.TRANSF, p.BAIXA, p.PICK, p.CORRE, p.BXAPICK, p.CRIAPICK`, codUsu)

	rows, err := c.executeQuery(sqlQuery)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("permissões não encontradas")
	}

	row := rows[0]
	toBool := func(val any) bool { return val.(string) == "S" }

	return &UserPermissions{
		ListaCodigos: fmt.Sprintf("%v", row[0]),
		ListaNomes:   fmt.Sprintf("%v", row[1]),
		CodUsu:       int(row[2].(float64)),
		Transf:       toBool(row[3]),
		Baixa:        toBool(row[4]),
		Pick:         toBool(row[5]),
		Corre:        toBool(row[6]),
		BxaPick:      toBool(row[7]),
		CriaPick:     toBool(row[8]),
	}, nil
}

func (c *Client) LoginUser(username, password string) (string, error) {
	sysToken, err := c.GetToken()
	if err != nil {
		return "", err
	}

	reqBody := mobileLoginRequest{ServiceName: "MobileLoginSP.login"}
	reqBody.RequestBody.NomUsu.Value = username
	reqBody.RequestBody.Interno.Value = password
	reqBody.RequestBody.KeepConnected.Value = "S"
	jsonData, _ := json.Marshal(reqBody)

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

	var result mobileLoginResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Status != "1" {
		return "", fmt.Errorf("credenciais inválidas")
	}
	return result.ResponseBody.JSessionID.Value, nil
}

// SearchItems busca itens no armazém (Otimizada)
func (c *Client) SearchItems(codArm int, filtro string) ([][]any, error) {
	var sqlBuilder strings.Builder

	// OTIMIZAÇÃO: Substituição da subquery correlacionada por LEFT JOIN
	// OTIMIZAÇÃO: Uso de /*+ ALL_ROWS */ para indicar ao Oracle que queremos o set completo rapidamente
	sqlBuilder.WriteString(fmt.Sprintf(`
		SELECT /*+ ALL_ROWS */
			ENDE.SEQEND, 
			ENDE.CODRUA, 
			ENDE.CODPRD, 
			ENDE.CODAPT, 
			ENDE.CODPROD, 
			PRO.DESCRPROD, 
			PRO.MARCA, 
			ENDE.DATVAL, 
			ENDE.QTDPRO, 
			ENDE.ENDPIC, 
			TO_CHAR(ENDE.QTDPRO) || ' ' || ENDE.CODVOL AS QTD_COMPLETA, 
			VOA.DERIVACAO
		FROM AD_CADEND ENDE 
		JOIN TGFPRO PRO ON PRO.CODPROD = ENDE.CODPROD
		LEFT JOIN (
			SELECT CODPROD, CODVOL, MAX(DESCRDANFE) AS DERIVACAO 
			FROM TGFVOA 
			GROUP BY CODPROD, CODVOL
		) VOA ON VOA.CODPROD = ENDE.CODPROD AND VOA.CODVOL = ENDE.CODVOL
		WHERE ENDE.CODARM = %d`, codArm))

	orderBy := " ORDER BY ENDE.ENDPIC DESC, ENDE.DATVAL ASC"

	if filtro != "" {
		filtroLimpo := strings.TrimSpace(filtro)
		isNumeric := regexp.MustCompile(`^\d+$`).MatchString(filtroLimpo)

		if isNumeric {
			filtroNum := sanitizeStringForSql(filtroLimpo)
			// Otimização: Acesso direto por ROWNUM 1 para subquery de verificação
			sqlBuilder.WriteString(fmt.Sprintf(` 
				AND (
					ENDE.SEQEND LIKE '%s%%' 
					OR ENDE.CODPROD = %s 
					OR ENDE.CODPROD = (
						SELECT CODPROD FROM AD_CADEND 
						WHERE SEQEND = %s AND CODARM = %d AND ROWNUM = 1
					)
				)`, filtroNum, filtroNum, filtroNum, codArm))
			
			orderBy = fmt.Sprintf(` ORDER BY CASE WHEN ENDE.SEQEND = %s THEN 0 ELSE 1 END, ENDE.ENDPIC DESC, ENDE.DATVAL ASC`, filtroNum)
		} else {
			palavrasChave := strings.Fields(filtroLimpo)
			if len(palavrasChave) > 0 {
				sqlBuilder.WriteString(" AND ")
				var condicoes []string
				for _, palavra := range palavrasChave {
					pUpper := sanitizeStringForSql(strings.ToUpper(palavra))
					// Mantém TRANSLATE para case/accent insensitive, inevitável sem index dedicado
					cond := fmt.Sprintf(`(
						TRANSLATE(UPPER(PRO.DESCRPROD), 'ÁÀÂÃÄÉÈÊËÍÌÎÏÓÒÔÕÖÚÙÇ', 'AAAAAEEEEIIIIOOOOOUUUUC') LIKE '%%%s%%' OR
						TRANSLATE(UPPER(PRO.MARCA), 'ÁÀÂÃÄÉÈÊËÍÌÎÏÓÒÔÕÖÚÙÇ', 'AAAAAEEEEIIIIOOOOOUUUUC') LIKE '%%%s%%'
					)`, pUpper, pUpper)
					condicoes = append(condicoes, cond)
				}
				sqlBuilder.WriteString(strings.Join(condicoes, " AND "))
			}
		}
	}

	sqlBuilder.WriteString(orderBy)
	return c.executeQuery(sqlBuilder.String())
}

func sanitizeStringForSql(s string) string {
	return strings.ReplaceAll(s, "'", "")
}