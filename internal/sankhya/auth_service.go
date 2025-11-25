package sankhya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// VerifyUserAccess valida se o usuário existe e tem permissão de app
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

// VerifyDevice verifica se o dispositivo está autorizado
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

// registerDevice (Privado) registra um novo dispositivo
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

// LoginUser realiza o login do usuário final (MobileLoginSP)
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