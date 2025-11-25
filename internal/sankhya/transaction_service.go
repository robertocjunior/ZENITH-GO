package sankhya

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// TransactionInput agrupa os dados necessários para processar uma transação
type TransactionInput struct {
	Type    string         // baixa, transferencia, picking, correcao
	Payload map[string]any // Dados flexíveis do body
	CodUsu  int
}

// ExecuteServiceWithCookie chama um serviço Sankhya usando o JSESSIONID do usuário (TransactionUrl)
func (c *Client) ExecuteServiceWithCookie(ctx context.Context, serviceName string, requestBody any, snkSessionId string) (*TransactionResponse, error) {
	url := fmt.Sprintf("%s/service.sbr?serviceName=%s&outputType=json", c.cfg.TransactionUrl, serviceName)

	// Envolve o body na estrutura padrão { "requestBody": ... } para chamadas diretas de serviço
	fullPayload := map[string]any{
		"requestBody": requestBody,
	}

	jsonData, err := json.Marshal(fullPayload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	// Define o Cookie JSESSIONID e Content-Type
	req.Header.Set("Cookie", fmt.Sprintf("JSESSIONID=%s", snkSessionId))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro de conexão com Sankhya Transaction: %w", err)
	}
	defer resp.Body.Close()

	var result TransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resposta da transação: %w", err)
	}

	// Status 1 = Sucesso, 2 = Sucesso com aviso
	if result.Status != "1" && result.Status != "2" {
		msg := result.StatusMessage
		if msg == "" {
			msg = "Erro desconhecido no Sankhya (Status " + result.Status + ")"
		}
		return nil, fmt.Errorf("erro na transação (%s): %s", serviceName, msg)
	}

	return &result, nil
}

// ExecuteServiceAsSystem chama um serviço Sankhya usando o Bearer Token do Sistema (ApiUrl)
func (c *Client) ExecuteServiceAsSystem(ctx context.Context, serviceName string, requestBody any) (*TransactionResponse, error) {
	// Obtém o token do sistema (renova se necessário)
	sysToken, err := c.GetToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/gateway/v1/mge/service.sbr?serviceName=%s&outputType=json", c.cfg.ApiUrl, serviceName)

	// O Gateway geralmente espera que o serviceName também esteja no corpo
	payload := ServiceRequest{
		ServiceName: serviceName,
		RequestBody: requestBody,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+sysToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro de conexão com Sankhya System API: %w", err)
	}
	defer resp.Body.Close()

	var result TransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resposta da System API: %w", err)
	}

	if result.Status != "1" {
		msg := result.StatusMessage
		if msg == "" {
			msg = "Erro desconhecido (Status " + result.Status + ")"
		}
		return nil, fmt.Errorf("erro na System API (%s): %s", serviceName, msg)
	}

	return &result, nil
}

// ExecuteTransaction orquestra a lógica baseada no tipo
func (c *Client) ExecuteTransaction(ctx context.Context, input TransactionInput, snkSessionId string) (string, error) {
	// 1. Validação de Permissões (Usa System Token)
	perms, err := c.GetUserPermissions(ctx, input.CodUsu)
	if err != nil {
		return "", fmt.Errorf("falha ao verificar permissões: %w", err)
	}

	hasPermission := false
	switch input.Type {
	case "baixa":
		hasPermission = perms.Baixa
	case "transferencia":
		hasPermission = perms.Transf
	case "picking":
		hasPermission = perms.Pick
	case "correcao":
		hasPermission = perms.Corre
	}

	if !hasPermission {
		return "", ErrPermissionDenied
	}

	// 2. Roteamento por Tipo
	if input.Type == "correcao" {
		return c.handleCorrecao(ctx, input, snkSessionId)
	} else {
		return c.handleMovimentacao(ctx, input, snkSessionId, perms)
	}
}

// handleCorrecao trata a lógica específica de correção de estoque
func (c *Client) handleCorrecao(ctx context.Context, input TransactionInput, snkSessionId string) (string, error) {
	payload := input.Payload
	codArm := int(payload["codarm"].(float64))
	sequencia := int(payload["sequencia"].(float64))
	newQuantity := payload["newQuantity"].(float64)

	// Busca dados do item (System Token - executeQuery)
	sqlItem := fmt.Sprintf(`
		SELECT 
			DEND.CODPROD, 
			DEND.CODVOL, 
			TO_CHAR(DEND.DATENT, 'DD/MM/YYYY') AS DATENT, 
			TO_CHAR(DEND.DATVAL, 'DD/MM/YYYY') AS DATVAL, 
			DEND.QTDPRO, 
			PRO.MARCA, 
			(SELECT MAX(V.DESCRDANFE) FROM TGFVOA V WHERE V.CODPROD = DEND.CODPROD AND V.CODVOL = DEND.CODVOL) AS DERIVACAO 
		FROM AD_CADEND DEND 
		JOIN TGFPRO PRO ON DEND.CODPROD = PRO.CODPROD 
		WHERE DEND.CODARM = %d AND DEND.SEQEND = %d`, codArm, sequencia)

	rows, err := c.executeQuery(ctx, sqlItem)
	if err != nil || len(rows) == 0 {
		return "", fmt.Errorf("item não encontrado para correção")
	}
	row := rows[0]

	// Helpers de extração
	getString := func(idx int) string {
		if row[idx] == nil {
			return ""
		}
		return fmt.Sprintf("%v", row[idx])
	}
	getFloat := func(idx int) float64 {
		if val, ok := row[idx].(float64); ok {
			return val
		}
		return 0
	}

	codProd := getString(0)
	codVol := getString(1)
	datEnt := getString(2)
	datVal := getString(3)
	qtdAnt := getFloat(4)
	marca := getString(5)
	deriv := getString(6)

	// Executa Script (ActionButtonsSP.executeScript) -> USER SESSION (Cookie)
	scriptBody := ExecuteScriptBody{}
	scriptBody.RunScript.ActionID = "97"
	scriptBody.RunScript.RefreshType = "SEL"
	scriptBody.RunScript.Params.Param = []ScriptParam{
		{Type: "S", ParamName: "CODPROD", Value: codProd},
		{Type: "S", ParamName: "CODVOL", Value: codVol},
		{Type: "F", ParamName: "QTDPRO", Value: newQuantity},
		{Type: "D", ParamName: "DATENT", Value: datEnt},
		{Type: "D", ParamName: "DATVAL", Value: datVal},
	}
	scriptBody.RunScript.Rows.Row = []ScriptRow{{
		Field: []ScriptField{
			{FieldName: "CODARM", Value: strconv.Itoa(codArm)},
			{FieldName: "SEQEND", Value: strconv.Itoa(sequencia)},
		},
	}}
	scriptBody.ClientEventList.ClientEvent = []map[string]string{{"$": "br.com.sankhya.actionbutton.clientconfirm"}}

	_, err = c.ExecuteServiceWithCookie(ctx, "ActionButtonsSP.executeScript", scriptBody, snkSessionId)
	if err != nil {
		return "", err
	}

	// Salva Histórico (DatasetSP.save) -> SYSTEM TOKEN (Bearer)
	// CORREÇÃO: Uso de "%.0f" para garantir string inteira ("50" e não "50.0")
	histBody := DatasetSaveBody{
		EntityName: "AD_HISTENDAPP",
		Fields:     []string{"CODARM", "SEQEND", "CODPROD", "CODVOL", "MARCA", "DERIV", "QUANT", "QATUAL", "CODUSU"},
		Records: []DatasetRecord{{
			Values: map[string]string{
				"0": strconv.Itoa(codArm),
				"1": strconv.Itoa(sequencia),
				"2": codProd,
				"3": codVol,
				"4": marca,
				"5": deriv,
				"6": fmt.Sprintf("%.0f", qtdAnt),    // Inteiro
				"7": fmt.Sprintf("%.0f", newQuantity), // Inteiro
				"8": strconv.Itoa(input.CodUsu),
			},
		}},
	}

	_, err = c.ExecuteServiceAsSystem(ctx, "DatasetSP.save", histBody)
	if err != nil {
		return fmt.Sprintf("Correção executada, mas erro no histórico: %v", err), nil
	}

	return "Correção executada com sucesso!", nil
}

// handleMovimentacao trata Baixa, Transferência e Picking
func (c *Client) handleMovimentacao(ctx context.Context, input TransactionInput, snkSessionId string, perms *UserPermissions) (string, error) {
	// 1. Cria Cabeçalho (AD_BXAEND)
	hoje := time.Now().Format("02/01/2006")
	headerBody := DatasetSaveBody{
		EntityName: "AD_BXAEND",
		Fields:     []string{"SEQBAI", "DATGER", "USUGER"},
		Records: []DatasetRecord{{
			Values: map[string]string{
				"1": hoje,
				"2": strconv.Itoa(input.CodUsu),
			},
		}},
	}

	resHeader, err := c.ExecuteServiceWithCookie(ctx, "DatasetSP.save", headerBody, snkSessionId)
	if err != nil {
		return "", fmt.Errorf("falha ao criar cabeçalho: %w", err)
	}

	if len(resHeader.ResponseBody.Result) == 0 || len(resHeader.ResponseBody.Result[0]) == 0 {
		return "", fmt.Errorf("não retornou SEQBAI")
	}
	seqBai := resHeader.ResponseBody.Result[0][0]

	// 2. Prepara Itens (AD_IBXEND)
	records := []DatasetRecord{}
	payload := input.Payload

	// Itens de baixa/movimentação geralmente aceitam decimais, mantemos %.3f
	fmtQtd := func(v any) string {
		f, _ := v.(float64)
		return fmt.Sprintf("%.3f", f)
	}

	if input.Type == "baixa" {
		origem := payload["origem"].(map[string]any)
		qtd := payload["quantidade"]

		if origem["endpic"] == "S" && !perms.BxaPick {
			return "", fmt.Errorf("sem permissão para baixar de picking")
		}

		records = append(records, DatasetRecord{
			Values: map[string]string{
				"0": seqBai,
				"1": fmt.Sprintf("%.0f", origem["codarm"]),
				"2": fmt.Sprintf("%.0f", origem["sequencia"]),
				"3": "", // null
				"4": "", // null
				"5": fmtQtd(qtd),
				"6": "S",
			},
		})
	} else {
		// Transferencia ou Picking
		origem := payload["origem"].(map[string]any)
		destino := payload["destino"].(map[string]any)

		if origem["endpic"] == "S" && !perms.BxaPick {
			return "", fmt.Errorf("sem permissão para mover de picking")
		}

		// Verifica destino (System Token)
		sqlDest := fmt.Sprintf("SELECT CODPROD, QTDPRO FROM AD_CADEND WHERE SEQEND = '%s' AND CODARM = %.0f",
			sanitizeStringForSql(destino["enderecoDestino"].(string)), destino["armazemDestino"])

		rowsDest, _ := c.executeQuery(ctx, sqlDest)

		// Simulação de Merge
		if len(rowsDest) > 0 {
			destProd := fmt.Sprintf("%v", rowsDest[0][0])
			destQtd := rowsDest[0][1].(float64)
			origProd := fmt.Sprintf("%.0f", origem["codprod"])

			if destProd == origProd {
				records = append(records, DatasetRecord{
					Values: map[string]string{
						"0": seqBai,
						"1": fmt.Sprintf("%.0f", destino["armazemDestino"]),
						"2": destino["enderecoDestino"].(string),
						"3": "",
						"4": "",
						"5": fmt.Sprintf("%.3f", destQtd),
						"6": "S",
					},
				})
			}
		}

		// Item Principal
		records = append(records, DatasetRecord{
			Values: map[string]string{
				"0": seqBai,
				"1": fmt.Sprintf("%.0f", origem["codarm"]),
				"2": fmt.Sprintf("%.0f", origem["sequencia"]),
				"3": fmt.Sprintf("%.0f", destino["armazemDestino"]),
				"4": destino["enderecoDestino"].(string),
				"5": fmtQtd(destino["quantidade"]),
				"6": "S",
			},
		})

		// Marca destino como picking se solicitado (User Session)
		createPick, _ := destino["criarPick"].(bool)
		if input.Type == "transferencia" && createPick && perms.CriaPick {
			updateBody := DatasetSaveBody{
				EntityName: "CADEND",
				StandAlone: false,
				Fields:     []string{"CODARM", "SEQEND", "ENDPIC"},
				Records: []DatasetRecord{{
					PK: map[string]string{
						"CODARM": fmt.Sprintf("%.0f", destino["armazemDestino"]),
						"SEQEND": destino["enderecoDestino"].(string),
					},
					Values: map[string]string{"2": "S"},
				}},
			}
			c.ExecuteServiceWithCookie(ctx, "DatasetSP.save", updateBody, snkSessionId)
		}
	}

	// 3. Salva Itens em Batch (User Session)
	if len(records) > 0 {
		itemsBody := DatasetSaveBody{
			EntityName: "AD_IBXEND",
			Fields:     []string{"SEQBAI", "CODARM", "SEQEND", "ARMDES", "ENDDES", "QTDPRO", "APP"},
			StandAlone: false,
			Records:    records,
		}
		_, err := c.ExecuteServiceWithCookie(ctx, "DatasetSP.save", itemsBody, snkSessionId)
		if err != nil {
			return "", fmt.Errorf("erro ao salvar itens: %w", err)
		}
	}

	// 4. Polling (System Token)
	populated := false
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		sqlPoll := fmt.Sprintf("SELECT COUNT(*) FROM AD_IBXEND WHERE SEQBAI = %s AND CODPROD IS NOT NULL", seqBai)
		rows, err := c.executeQuery(ctx, sqlPoll)
		if err == nil && len(rows) > 0 {
			count, _ := strconv.Atoi(fmt.Sprintf("%v", rows[0][0]))
			if count >= len(records) {
				populated = true
				break
			}
		}
	}
	if !populated {
		return "", fmt.Errorf("timeout: sistema não processou itens a tempo")
	}

	// 5. Executa STP Finalizadora (User Session)
	stpBody := ExecuteSTPBody{}
	stpBody.StpCall.ActionID = "20"
	stpBody.StpCall.ProcName = "NIC_STP_BAIXA_END"
	stpBody.StpCall.RootEntity = "AD_BXAEND"
	stpBody.StpCall.Rows.Row = []ScriptRow{{
		Field: []ScriptField{{FieldName: "SEQBAI", Value: seqBai}},
	}}

	resp, err := c.ExecuteServiceWithCookie(ctx, "ActionButtonsSP.executeSTP", stpBody, snkSessionId)
	if err != nil {
		return "", fmt.Errorf("erro na procedure final: %w", err)
	}

	if resp.StatusMessage != "" {
		return resp.StatusMessage, nil
	}

	return "Operação concluída com sucesso!", nil
}