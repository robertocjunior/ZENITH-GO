package sankhya

import (
	"bytes"
	"context"
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
	ErrItemNotFound          = errors.New("item não encontrado")
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

// ItemDetail define o formato padronizado da resposta de detalhes
type ItemDetail struct {
	CodArm      int     `json:"codArm"`
	SeqEnd      int     `json:"seqEnd"`
	CodRua      string  `json:"codRua"`
	CodPrd      int     `json:"codPrd"`
	CodApt      string  `json:"codApt"`
	CodProd     int     `json:"codProd"`
	DescrProd   string  `json:"descrProd"`
	Marca       string  `json:"marca"`
	DatVal      string  `json:"datVal"`
	QtdPro      float64 `json:"qtdPro"`
	EndPic      string  `json:"endPic"`
	NumDoc      int     `json:"numDoc"`
	QtdCompleta string  `json:"qtdCompleta"`
	Derivacao   string  `json:"derivacao"`
}

// SearchItemResult define o formato padronizado da resposta de busca
type SearchItemResult struct {
	SeqEnd      int     `json:"seqEnd"`
	CodRua      string  `json:"codRua"`
	CodPrd      int     `json:"codPrd"`
	CodApt      string  `json:"codApt"`
	CodProd     int     `json:"codProd"`
	DescrProd   string  `json:"descrProd"`
	Marca       string  `json:"marca"`
	DatVal      string  `json:"datVal"`
	QtdPro      float64 `json:"qtdPro"`
	EndPic      string  `json:"endPic"`
	QtdCompleta string  `json:"qtdCompleta"`
	Derivacao   string  `json:"derivacao"`
}

// PickingLocation define o resultado da rota get-picking-locations
type PickingLocation struct {
	SeqEnd    int    `json:"seqEnd"`
	DescrProd string `json:"descrProd"`
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
func (c *Client) Authenticate(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

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

func (c *Client) GetToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.bearerToken != "" && time.Now().Before(c.tokenExpiry) {
		token := c.bearerToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	if err := c.Authenticate(ctx); err != nil {
		return "", err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bearerToken, nil
}

// executeQuery executa SQL
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

	var result dbExplorerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erro decodificando query: %w", err)
	}

	return result.ResponseBody.Rows, nil
}

// VerifyUserAccess
func (c *Client) VerifyUserAccess(ctx context.Context, username string) (float64, error) {
	safeUsername := sanitizeStringForSql(strings.ToUpper(username))

	sqlQuery := fmt.Sprintf(`
		SELECT 
			U.CODUSU, 
			CASE 
				WHEN EXISTS (SELECT 1 FROM AD_APPPERM P WHERE P.CODUSU = U.CODUSU) THEN 'TRUE' 
				ELSE 'FALSE' 
			END AS PERMITIDO 
		FROM TSIUSU U 
		WHERE U.NOMEUSU = '%s'`, safeUsername)

	rows, err := c.executeQuery(ctx, sqlQuery)
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

// VerifyDevice
func (c *Client) VerifyDevice(ctx context.Context, codUsu int, deviceToken string) error {
	safeToken := sanitizeStringForSql(deviceToken)
	
	sqlQuery := fmt.Sprintf(`
		SELECT DEVICETOKEN, CODUSU, ATIVO 
		FROM AD_DISPAUT 
		WHERE CODUSU = %d AND DEVICETOKEN = '%s'`, codUsu, safeToken)

	rows, err := c.executeQuery(ctx, sqlQuery)
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		sysToken, _ := c.GetToken(ctx)
		if regErr := c.registerDevice(ctx, sysToken, codUsu, deviceToken); regErr != nil {
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

func (c *Client) registerDevice(ctx context.Context, token string, codUsu int, deviceToken string) error {
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
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
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

// GetUserPermissions
func (c *Client) GetUserPermissions(ctx context.Context, codUsu int) (*UserPermissions, error) {
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

	rows, err := c.executeQuery(ctx, sqlQuery)
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

func (c *Client) LoginUser(ctx context.Context, username, password string) (string, error) {
	sysToken, err := c.GetToken(ctx)
	if err != nil {
		return "", err
	}

	reqBody := mobileLoginRequest{ServiceName: "MobileLoginSP.login"}
	reqBody.RequestBody.NomUsu.Value = username
	reqBody.RequestBody.Interno.Value = password
	reqBody.RequestBody.KeepConnected.Value = "S"
	jsonData, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("%s/gateway/v1/mge/service.sbr?serviceName=MobileLoginSP.login&outputType=json", c.cfg.ApiUrl)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
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

// GetItemDetails
func (c *Client) GetItemDetails(ctx context.Context, codArm int, sequencia string) (*ItemDetail, error) {
	safeSeq := sanitizeStringForSql(sequencia)
	sql := fmt.Sprintf(`SELECT * FROM V_WMS_ITEM_DETALHES WHERE CODARM = %d AND SEQEND = '%s'`, codArm, safeSeq)
	
	rows, err := c.executeQuery(ctx, sql)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, ErrItemNotFound
	}

	row := rows[0]

	// Helpers
	getInt := func(i int) int {
		if i >= len(row) || row[i] == nil { return 0 }
		if f, ok := row[i].(float64); ok { return int(f) }
		return 0
	}
	getFloat := func(i int) float64 {
		if i >= len(row) || row[i] == nil { return 0 }
		if f, ok := row[i].(float64); ok { return f }
		return 0
	}
	getString := func(i int) string {
		if i >= len(row) || row[i] == nil { return "" }
		return fmt.Sprintf("%v", row[i])
	}

	item := &ItemDetail{
		CodArm:      getInt(0),
		SeqEnd:      getInt(1),
		CodRua:      getString(2),
		CodPrd:      getInt(3),
		CodApt:      getString(4),
		CodProd:     getInt(5),
		DescrProd:   getString(6),
		Marca:       getString(7),
		DatVal:      getString(8),
		QtdPro:      getFloat(9),
		EndPic:      getString(10),
		NumDoc:      getInt(11),
		QtdCompleta: getString(12),
		Derivacao:   getString(13),
	}

	return item, nil
}

// GetPickingLocations busca locais de picking alternativos
func (c *Client) GetPickingLocations(ctx context.Context, codArm int, codProd int, sequenciaExclude int) ([]PickingLocation, error) {
	// Como os parâmetros são inteiros, %d é seguro contra injeção de SQL
	sql := fmt.Sprintf(`
		SELECT ENDE.SEQEND, PRO.DESCRPROD 
		FROM AD_CADEND ENDE 
		JOIN TGFPRO PRO ON ENDE.CODPROD = PRO.CODPROD 
		WHERE ENDE.CODARM = %d 
		AND ENDE.CODPROD = %d 
		AND ENDE.ENDPIC = 'S' 
		AND ENDE.SEQEND <> %d 
		ORDER BY ENDE.SEQEND`, codArm, codProd, sequenciaExclude)

	rows, err := c.executeQuery(ctx, sql)
	if err != nil {
		return nil, err
	}

	var results []PickingLocation
	for _, row := range rows {
		// Helper para extração segura
		getInt := func(i int) int {
			if i >= len(row) || row[i] == nil { return 0 }
			if f, ok := row[i].(float64); ok { return int(f) }
			return 0
		}
		getString := func(i int) string {
			if i >= len(row) || row[i] == nil { return "" }
			return fmt.Sprintf("%v", row[i])
		}

		results = append(results, PickingLocation{
			SeqEnd:    getInt(0),
			DescrProd: getString(1),
		})
	}
	
	return results, nil
}

// SearchItems
func (c *Client) SearchItems(ctx context.Context, codArm int, filtro string) ([]SearchItemResult, error) {
	var sqlBuilder strings.Builder
	
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
			filtroSafe := sanitizeStringForSql(filtroLimpo)
			sqlBuilder.WriteString(fmt.Sprintf(` 
				AND (
					ENDE.SEQEND LIKE '%s%%' 
					OR ENDE.CODPROD = %s 
					OR ENDE.CODPROD = (
						SELECT CODPROD FROM AD_CADEND 
						WHERE SEQEND = %s AND CODARM = %d AND ROWNUM = 1
					)
				)`, filtroSafe, filtroSafe, filtroSafe, codArm))
			
			orderBy = fmt.Sprintf(` ORDER BY CASE WHEN ENDE.SEQEND = %s THEN 0 ELSE 1 END, ENDE.ENDPIC DESC, ENDE.DATVAL ASC`, filtroSafe)
		} else {
			palavrasChave := strings.Fields(filtroLimpo)
			if len(palavrasChave) > 0 {
				sqlBuilder.WriteString(" AND ")
				var condicoes []string
				for _, palavra := range palavrasChave {
					pUpper := sanitizeStringForSql(strings.ToUpper(palavra))
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
	
	rows, err := c.executeQuery(ctx, sqlBuilder.String())
	if err != nil {
		return nil, err
	}

	var results []SearchItemResult
	for _, row := range rows {
		// Helpers
		getInt := func(i int) int {
			if i >= len(row) || row[i] == nil { return 0 }
			if f, ok := row[i].(float64); ok { return int(f) }
			return 0
		}
		getFloat := func(i int) float64 {
			if i >= len(row) || row[i] == nil { return 0 }
			if f, ok := row[i].(float64); ok { return f }
			return 0
		}
		getString := func(i int) string {
			if i >= len(row) || row[i] == nil { return "" }
			return fmt.Sprintf("%v", row[i])
		}

		results = append(results, SearchItemResult{
			SeqEnd:      getInt(0),
			CodRua:      getString(1),
			CodPrd:      getInt(2),
			CodApt:      getString(3),
			CodProd:     getInt(4),
			DescrProd:   getString(5),
			Marca:       getString(6),
			DatVal:      getString(7),
			QtdPro:      getFloat(8),
			EndPic:      getString(9),
			QtdCompleta: getString(10),
			Derivacao:   getString(11),
		})
	}

	return results, nil
}

func sanitizeStringForSql(s string) string {
	return strings.ReplaceAll(s, "'", "")
}