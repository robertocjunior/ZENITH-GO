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

// Helpers para conversão segura de tipos
func safeFloat64(v any) float64 {
	if v == nil {
		return 0
	}
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

func safeString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// ExecuteServiceWithCookie chama um serviço Sankhya usando o JSESSIONID do usuário
func (c *Client) ExecuteServiceWithCookie(ctx context.Context, serviceName string, requestBody any, snkSessionId string) (*TransactionResponse, error) {
	url := fmt.Sprintf("%s/service.sbr?serviceName=%s&outputType=json", c.cfg.TransactionUrl, serviceName)

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

	if result.Status != "1" && result.Status != "2" {
		msg := result.StatusMessage
		if msg == "" {
			msg = "Erro desconhecido no Sankhya (Status " + result.Status + ")"
		}
		return nil, fmt.Errorf("erro na transação (%s): %s", serviceName, msg)
	}

	return &result, nil
}

// ExecuteServiceAsSystem chama um serviço Sankhya usando o Bearer Token do Sistema
func (c *Client) ExecuteServiceAsSystem(ctx context.Context, serviceName string, requestBody any) (*TransactionResponse, error) {
	sysToken, err := c.GetToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/gateway/v1/mge/service.sbr?serviceName=%s&outputType=json", c.cfg.ApiUrl, serviceName)

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

	if input.Type == "correcao" {
		return c.handleCorrecao(ctx, input, snkSessionId)
	} else if input.Type == "picking" {
		return c.handlePicking(ctx, input, snkSessionId, perms)
	} else {
		return c.handleMovimentacao(ctx, input, snkSessionId, perms)
	}
}

// handleCorrecao trata a lógica específica de correção de estoque
func (c *Client) handleCorrecao(ctx context.Context, input TransactionInput, snkSessionId string) (string, error) {
	payload := input.Payload
	codArm := int(safeFloat64(payload["codarm"]))
	sequencia := int(safeFloat64(payload["sequencia"]))
	newQuantity := safeFloat64(payload["newQuantity"])

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

	codProd := safeString(row[0])
	codVol := safeString(row[1])
	datEnt := safeString(row[2])
	datVal := safeString(row[3])
	qtdAnt := safeFloat64(row[4])
	marca := safeString(row[5])
	deriv := safeString(row[6])

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
				"6": fmt.Sprintf("%.0f", qtdAnt),
				"7": fmt.Sprintf("%.0f", newQuantity),
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

// getOriginData busca CODPROD e ENDPIC da origem
func (c *Client) getOriginData(ctx context.Context, codArm int, sequencia int) (string, string, error) {
	sql := fmt.Sprintf(`SELECT CODPROD, ENDPIC FROM AD_CADEND WHERE CODARM = %d AND SEQEND = %d`, codArm, sequencia)
	rows, err := c.executeQuery(ctx, sql)
	if err != nil {
		return "", "", fmt.Errorf("erro ao consultar dados da origem: %w", err)
	}
	if len(rows) == 0 {
		return "", "", fmt.Errorf("item de origem não encontrado no estoque")
	}

	codProd := fmt.Sprintf("%.0f", safeFloat64(rows[0][0]))
	endPic := safeString(rows[0][1])

	return codProd, endPic, nil
}

// handlePicking: Lógica exclusiva para Picking (Desacoplada)
func (c *Client) handlePicking(ctx context.Context, input TransactionInput, snkSessionId string, perms *UserPermissions) (string, error) {
	payload := input.Payload

	// 1. Parse Origem
	var origemCodArm, origemSeq int
	if origemMap, ok := payload["origem"].(map[string]any); ok {
		origemCodArm = int(safeFloat64(origemMap["codarm"]))
		origemSeq = int(safeFloat64(origemMap["sequencia"]))
	} else {
		return "", fmt.Errorf("payload inválido: origem não encontrada")
	}

	// 2. Parse Destino
	var destCodArm int
	var destSeq string
	var destQtd float64
	if destMap, ok := payload["destino"].(map[string]any); ok {
		destCodArm = int(safeFloat64(destMap["armazemDestino"]))
		destSeq = safeString(destMap["enderecoDestino"])
		destQtd = safeFloat64(destMap["quantidade"])
	} else {
		return "", fmt.Errorf("payload inválido: destino não encontrado")
	}

	// 3. Validação da Origem (Server-Side)
	serverCodProd, serverEndPic, err := c.getOriginData(ctx, origemCodArm, origemSeq)
	if err != nil {
		return "", err
	}

	if serverEndPic == "S" && !perms.BxaPick {
		return "", fmt.Errorf("permissão negada: origem é Picking e usuário não tem permissão BXAPICK")
	}

	// 4. Validação e Consulta do Destino (Server-Side)
	// Buscamos QTDPRO também para poder fazer a baixa se necessário
	sqlDest := fmt.Sprintf("SELECT CODPROD, QTDPRO FROM AD_CADEND WHERE CODARM = %d AND SEQEND = '%s'", destCodArm, sanitizeStringForSql(destSeq))
	rowsDest, err := c.executeQuery(ctx, sqlDest)
	if err != nil {
		return "", fmt.Errorf("erro ao consultar destino: %w", err)
	}

	// Lista de registros para salvar em AD_IBXEND (Itens da transação)
	records := []DatasetRecord{}
	fmtQtd := func(v float64) string { return fmt.Sprintf("%.3f", v) }

	// 5. Cria Cabeçalho (AD_BXAEND)
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

	// 6. Lógica de Limpeza (Baixa) do Destino
	if len(rowsDest) > 0 {
		destProd := fmt.Sprintf("%.0f", safeFloat64(rowsDest[0][0]))
		destCurrentQtd := safeFloat64(rowsDest[0][1])

		// Se o destino tem produto e é o MESMO da origem, fazemos a baixa para "limpar" (zerar para repor)
		// Se for diferente, bloqueamos (regra de negócio para evitar mistura)
		if destProd != "0" {
			if destProd == serverCodProd {
				// Gera registro de BAIXA (Saída) do que já existe no destino
				records = append(records, DatasetRecord{
					Values: map[string]string{
						"0": seqBai,
						"1": fmt.Sprintf("%d", destCodArm),
						"2": destSeq,
						"3": "", // Destino nulo = Baixa
						"4": "",
						"5": fmtQtd(destCurrentQtd),
						"6": "S",
					},
				})
			} else {
				return "", fmt.Errorf("operação negada: endereço de destino contém produto diferente (%s) da origem (%s)", destProd, serverCodProd)
			}
		}
	}

	// 7. Lógica de Transferência (Origem -> Destino)
	records = append(records, DatasetRecord{
		Values: map[string]string{
			"0": seqBai,
			"1": fmt.Sprintf("%d", origemCodArm),
			"2": fmt.Sprintf("%d", origemSeq),
			"3": fmt.Sprintf("%d", destCodArm),
			"4": destSeq,
			"5": fmtQtd(destQtd),
			"6": "S",
		},
	})

	// 8. Salva Itens (AD_IBXEND)
	itemsBody := DatasetSaveBody{
		EntityName: "AD_IBXEND",
		Fields:     []string{"SEQBAI", "CODARM", "SEQEND", "ARMDES", "ENDDES", "QTDPRO", "APP"},
		StandAlone: false,
		Records:    records,
	}

	_, err = c.ExecuteServiceWithCookie(ctx, "DatasetSP.save", itemsBody, snkSessionId)
	if err != nil {
		return "", fmt.Errorf("erro ao salvar itens de picking: %w", err)
	}

	// 9. Define Destino como Picking (ENDPIC = 'S')
	updateBody := DatasetSaveBody{
		EntityName: "CADEND",
		StandAlone: false,
		Fields:     []string{"CODARM", "SEQEND", "ENDPIC"},
		Records: []DatasetRecord{{
			PK: map[string]string{
				"CODARM": fmt.Sprintf("%d", destCodArm),
				"SEQEND": destSeq,
			},
			Values: map[string]string{"2": "S"}, // Define ENDPIC='S'
		}},
	}
	c.ExecuteServiceWithCookie(ctx, "DatasetSP.save", updateBody, snkSessionId)
	// Erros aqui não são críticos para travar a movimentação, mas idealmente logaríamos warning

	// 10. Polling
	populated := false
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		sqlPoll := fmt.Sprintf("SELECT COUNT(*) FROM AD_IBXEND WHERE SEQBAI = %s AND CODPROD IS NOT NULL", seqBai)
		rows, err := c.executeQuery(ctx, sqlPoll)
		if err == nil && len(rows) > 0 {
			count := int(safeFloat64(rows[0][0]))
			if count >= len(records) {
				populated = true
				break
			}
		}
	}
	if !populated {
		return "", fmt.Errorf("timeout: sistema não processou o picking a tempo")
	}

	// 11. Finaliza
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
	return "Picking realizado com sucesso!", nil
}

// handleMovimentacao: Baixa e Transferência (Sem Picking)
func (c *Client) handleMovimentacao(ctx context.Context, input TransactionInput, snkSessionId string, perms *UserPermissions) (string, error) {
	payload := input.Payload

	// Validação e Busca de Dados da Origem (Server-Side)
	var origemCodArm, origemSeq int
	if origemMap, ok := payload["origem"].(map[string]any); ok {
		origemCodArm = int(safeFloat64(origemMap["codarm"]))
		origemSeq = int(safeFloat64(origemMap["sequencia"]))
	} else {
		return "", fmt.Errorf("payload inválido: dados de origem não encontrados")
	}

	serverCodProd, serverEndPic, err := c.getOriginData(ctx, origemCodArm, origemSeq)
	if err != nil {
		return "", err
	}

	if serverEndPic == "S" && !perms.BxaPick {
		return "", fmt.Errorf("permissão negada: origem é Picking e usuário não tem permissão BXAPICK")
	}

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

	records := []DatasetRecord{}
	fmtQtd := func(v any) string { return fmt.Sprintf("%.3f", safeFloat64(v)) }

	if input.Type == "baixa" {
		qtd := payload["quantidade"]
		records = append(records, DatasetRecord{
			Values: map[string]string{
				"0": seqBai,
				"1": fmt.Sprintf("%d", origemCodArm),
				"2": fmt.Sprintf("%d", origemSeq),
				"3": "",
				"4": "",
				"5": fmtQtd(qtd),
				"6": "S",
			},
		})
	} else {
		// Transferência
		destino := payload["destino"].(map[string]any)
		destCodArm := int(safeFloat64(destino["armazemDestino"]))
		destSeq := safeString(destino["enderecoDestino"])
		destQtdUser := destino["quantidade"]

		// Verifica destino para Consolidação (Merge)
		sqlDest := fmt.Sprintf("SELECT CODPROD, QTDPRO FROM AD_CADEND WHERE SEQEND = '%s' AND CODARM = %d",
			sanitizeStringForSql(destSeq), destCodArm)

		rowsDest, _ := c.executeQuery(ctx, sqlDest)

		if len(rowsDest) > 0 {
			destProd := fmt.Sprintf("%.0f", safeFloat64(rowsDest[0][0]))
			destQtd := safeFloat64(rowsDest[0][1])

			// Se mesmo produto, baixa destino primeiro (Merge)
			if destProd == serverCodProd {
				records = append(records, DatasetRecord{
					Values: map[string]string{
						"0": seqBai,
						"1": fmt.Sprintf("%d", destCodArm),
						"2": destSeq,
						"3": "",
						"4": "",
						"5": fmt.Sprintf("%.3f", destQtd),
						"6": "S",
					},
				})
			}
		}

		// Item Principal (Origem -> Destino)
		records = append(records, DatasetRecord{
			Values: map[string]string{
				"0": seqBai,
				"1": fmt.Sprintf("%d", origemCodArm),
				"2": fmt.Sprintf("%d", origemSeq),
				"3": fmt.Sprintf("%d", destCodArm),
				"4": destSeq,
				"5": fmtQtd(destQtdUser),
				"6": "S",
			},
		})

		// Marca destino como picking (Transferência apenas)
		createPick, _ := destino["criarPick"].(bool)
		if input.Type == "transferencia" && createPick && perms.CriaPick {
			updateBody := DatasetSaveBody{
				EntityName: "CADEND",
				StandAlone: false,
				Fields:     []string{"CODARM", "SEQEND", "ENDPIC"},
				Records: []DatasetRecord{{
					PK: map[string]string{
						"CODARM": fmt.Sprintf("%d", destCodArm),
						"SEQEND": destSeq,
					},
					Values: map[string]string{"2": "S"},
				}},
			}
			c.ExecuteServiceWithCookie(ctx, "DatasetSP.save", updateBody, snkSessionId)
		}
	}

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

	populated := false
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		sqlPoll := fmt.Sprintf("SELECT COUNT(*) FROM AD_IBXEND WHERE SEQBAI = %s AND CODPROD IS NOT NULL", seqBai)
		rows, err := c.executeQuery(ctx, sqlPoll)
		if err == nil && len(rows) > 0 {
			count := int(safeFloat64(rows[0][0]))
			if count >= len(records) {
				populated = true
				break
			}
		}
	}
	if !populated {
		return "", fmt.Errorf("timeout: sistema não processou itens a tempo")
	}

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