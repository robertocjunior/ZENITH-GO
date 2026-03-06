package sankhya

import (
	"context"
	"fmt"
	"time"
)

// GetRomaneios executa a consulta de fechamentos de carga com peso total e status de conferência
func (c *Client) GetRomaneios(ctx context.Context, dataFiltro string) ([]RomaneioResult, error) {
	safeData := sanitizeStringForSql(dataFiltro)

	sql := fmt.Sprintf(`
		SELECT FEC.NUFECHAMENTO AS FECHAMENTO,
		       TO_CHAR(FEC.DTFECHAMENTO, 'DD/MM/YYYY') AS DATA,
		       PAR.NOMEPARC AS MOTORISTA,
		       COM.PESO_TOTAL AS PESO,
		       VEI.PLACA AS PLACA,
		       VEI.AD_NUMINT || '-' || VEI.MARCAMODELO AS VEICULO,
		       VEI.AD_QTDPALLET AS PALETES,
		       FCAB.CODUSU,
		       USU.NOMEUSUCPLT AS NOMEUSU,
		       FCAB.STATUS
		  FROM AD_FECCAR FEC
		  JOIN AD_FECMOT MOT ON FEC.NUFECHAMENTO = MOT.NUFECHAMENTO
		  JOIN TGFPAR PAR ON MOT.CODPARC = PAR.CODPARC
		  JOIN TGFROT ROT ON FEC.CODROTA = ROT.CODROTA
		  JOIN TGFVEI VEI ON FEC.CODVEICULO = VEI.CODVEICULO
		  JOIN AD_ZNTCONFCAB FCAB ON FCAB.NUFECHAMENTO = FEC.NUFECHAMENTO
		  LEFT JOIN TSIUSU USU ON USU.CODUSU = FCAB.CODUSU
		  LEFT JOIN (
		        SELECT NUFECHAMENTO, 
		               ROUND(SUM(PESOBRUTO), 3) AS PESO_TOTAL
		          FROM AD_FECCOM
		         GROUP BY NUFECHAMENTO
		  ) COM ON FEC.NUFECHAMENTO = COM.NUFECHAMENTO
		 WHERE MOT.TIPO = 'M'
		   AND NVL(FEC.STATUS, 'A') <> 'A'
		   AND TRUNC(FEC.DTFECHAMENTO) = TO_DATE('%s', 'DD/MM/YYYY')
		   AND FCAB.CONFERIDO <> 'S'
		 ORDER BY FEC.NUFECHAMENTO DESC`, safeData)

	rows, err := c.executeQuery(ctx, sql)
	if err != nil {
		return nil, err
	}

	var results []RomaneioResult
	for _, row := range rows {
		// Helpers para evitar Panic com Nulls
		getInt := func(v any) int {
			if f, ok := v.(float64); ok { return int(f) }
			return 0
		}
		getString := func(v any) string {
			if v == nil { return "" }
			return fmt.Sprintf("%v", v)
		}
		getFloat := func(v any) float64 {
			if f, ok := v.(float64); ok { return f }
			return 0
		}

		results = append(results, RomaneioResult{
			Fechamento:  getInt(row[0]),
			Data:        getString(row[1]),
			Motorista:   getString(row[2]),
			Peso:        getFloat(row[3]),
			Placa:       getString(row[4]),
			Veiculo:     getString(row[5]),
			Paletes:     getFloat(row[6]),
			CodUsuario:  getInt(row[7]),    // Mapeando CODUSU
			NomeUsuario: getString(row[8]), // Mapeando NOMEUSU
			Status:      getString(row[9]), // Mapeando STATUS
		})
	}

	return results, nil
}

// GetRomaneioDetalhes busca os itens do romaneio com arredondamento corrigido
func (c *Client) GetRomaneioDetalhes(ctx context.Context, nuFec int) (*RomaneioDetalheResponse, error) {
	sql := fmt.Sprintf(`
SELECT 
    FEC.NUFECHAMENTO AS FECHAMENTO, 
    CAB.NUUNICO, 
    TO_CHAR(FEC.DTFECHAMENTO, 'DD/MM/YYYY') AS DATA, 
    PAR.NOMEPARC AS MOTORISTA, 
    COM_PESO.PESO_TOTAL AS PESO,
    VEI.PLACA AS PLACA, 
    VEI.AD_NUMINT || '-' || VEI.MARCAMODELO AS VEICULO, 
    VEI.AD_QTDPALLET AS PALETES,
    CAB.CODUSU, 
    USU.NOMEUSUCPLT AS NOMEUSU, 
    CAB.STATUS AS STATUS_CONF,
    -- DADOS DO ITEM
    CONF.TIPO,
    -- Se tiver CODPROD, mostra, se não tenta mostrar do TMS, ou vazio
    NVL(TO_CHAR(CONF.CODPROD), ' ') AS CODPROD,
    
    -- CORREÇÃO DA DESCRIÇÃO E MARCA:
    -- Usa a descrição salva na tabela (CONF). Se não tiver, tenta do cadastro (PRO).
    -- Concatena Marca apenas UMA vez.
    NVL(CONF.DESCRPROD, PRO.DESCRPROD) || 
    CASE WHEN CONF.MARCA IS NOT NULL THEN ' ' || CONF.MARCA ELSE '' END AS DESCRPROD,
    
    CONF.CODVOL,
    
    -- Referencia: Se não tiver no PRO (TMS), tenta buscar da tabela CONF se tivesse salvo (hoje não salva referencia texto no insert, mas pode ajustar se precisar)
    NVL(PRO.REFERENCIA, ' ') AS REFERENCIA,
    
    (SELECT SUBSTR(CODBARRA, -4) 
       FROM TGFVOA V 
      WHERE V.CODPROD = CONF.CODPROD 
        AND V.CODVOL = CONF.CODVOL 
        AND ROWNUM = 1) AS CODBARRA4DIG,
        
    CONF.QUANT AS QTDNEG,
    
    -- Peso: Se for TMS (Sem CODPROD), o peso unitário não vem do PRO. 
    -- Se precisar do peso bruto do TMS, precisaria salvar na tabela também. 
    -- Abaixo mantém logica original para itens de estoque.
    ROUND(CONF.QUANT * NVL(PRO.PESOBRUTO, 0), 3) AS PESOBRUTO,
    
    NVL(CONF.CONFERIDO, 'N') AS CONFERIDO,
    CONF.NUMREG,
    (SELECT LISTAGG(CODBARRA, ', ') WITHIN GROUP (ORDER BY CODBARRA)
       FROM TGFVOA 
      WHERE CODPROD = CONF.CODPROD
        AND CODBARRA IS NOT NULL) AS LISTA_BARRAS
FROM AD_ZNTITEMCONF CONF
JOIN AD_ZNTCONFCAB CAB ON CONF.NUUNICO = CAB.NUUNICO
JOIN AD_FECCAR FEC ON CONF.NUFECHAMENTO = FEC.NUFECHAMENTO
JOIN AD_FECMOT MOT ON FEC.NUFECHAMENTO = MOT.NUFECHAMENTO
JOIN TGFPAR PAR ON MOT.CODPARC = PAR.CODPARC
JOIN TGFVEI VEI ON FEC.CODVEICULO = VEI.CODVEICULO
LEFT JOIN TSIUSU USU ON USU.CODUSU = CAB.CODUSU
LEFT JOIN (
    SELECT NUFECHAMENTO, ROUND(SUM(PESOBRUTO), 3) AS PESO_TOTAL
      FROM AD_FECCOM GROUP BY NUFECHAMENTO
) COM_PESO ON FEC.NUFECHAMENTO = COM_PESO.NUFECHAMENTO
LEFT JOIN TGFPRO PRO ON CONF.CODPROD = PRO.CODPROD
WHERE MOT.TIPO = 'M' 
  AND FEC.NUFECHAMENTO = %d
ORDER BY CONF.NUMREG`, nuFec)

	rows, err := c.executeQuery(ctx, sql)
	if err != nil {
		return nil, err
	}
	// Permite retornar lista vazia se a conferência ainda não foi populada pela trigger,
	// mas mantemos o erro caso não ache nada e isso seja crítico (opcional: remover if abaixo se quiser vazio)
	if len(rows) == 0 {
		return nil, fmt.Errorf("nenhum registro de conferência encontrado para o fechamento %d", nuFec)
	}

	// Helpers (Null Safety)
	getFloat := func(v any) float64 {
		if v == nil { return 0.0 }
		if f, ok := v.(float64); ok { return f }
		return 0.0
	}
	getString := func(v any) string {
		if v == nil { return "" }
		return fmt.Sprintf("%v", v)
	}
	getInt := func(v any) int {
		if v == nil { return 0 }
		if f, ok := v.(float64); ok { return int(f) }
		return 0
	}

	// Mapeia o cabeçalho (0-10) usando a primeira linha
	res := &RomaneioDetalheResponse{
		Fechamento:        getInt(rows[0][0]),
		NuUnico:           getInt(rows[0][1]),
		Data:              getString(rows[0][2]),
		Motorista:         getString(rows[0][3]),
		PesoTotal:         getFloat(rows[0][4]),
		Placa:             getString(rows[0][5]),
		Veiculo:           getString(rows[0][6]),
		Paletes:           getFloat(rows[0][7]),
		CodUsuario:        getInt(rows[0][8]),
		NomeUsuario:       getString(rows[0][9]),
		StatusConferencia: getString(rows[0][10]),
		Produtos:          []RomaneioItem{},
	}

	// Mapeia os itens (11-21)
	for _, row := range rows {
		res.Produtos = append(res.Produtos, RomaneioItem{
			Tipo:          getString(row[11]),
			CodigoProduto: getString(row[12]),
			Descricao:     getString(row[13]),
			Unidade:       getString(row[14]),
			Referencia:    getString(row[15]),
			CodigoBarras4: getString(row[16]),
			Quantidade:    getFloat(row[17]),
			PesoBruto:     getFloat(row[18]),
			Conferido:     getString(row[19]),
			NumReg:        getInt(row[20]),
			ListaBarras:   getString(row[21]),
		})
	}

	return res, nil
}

func (c *Client) IniciarConferencia(ctx context.Context, nuUnico int, snkSessionId string) (*TransactionResponse, error) {
	// Formata a data atual: DD/MM/YYYY HH:mm:00
	dataAtual := time.Now().Format("02/01/2006 15:04:00")

	// Montagem do Payload para o ActionButtonsSP.executeSTP
	// Nota: ExecuteServiceWithCookie já encapsula isso dentro de um "requestBody",
	// então montamos apenas o conteúdo interno.
	requestBody := map[string]any{
		"stpCall": map[string]any{
			"actionID":    "172",
			"procName":    "STP_INICIAR_CONF_ZNT",
			"rootEntity":  "AD_ZNTCONFCAB",
			"refreshType": "SEL",
			"params": map[string]any{
				"param": []map[string]any{
					{
						"type":      "D",
						"paramName": "DTINICONF",
						"$":         dataAtual,
					},
				},
			},
			"rows": map[string]any{
				"row": []map[string]any{
					{
						"field": []map[string]any{
							{
								"fieldName": "NUUNICO",
								"$":         fmt.Sprintf("%d", nuUnico),
							},
						},
					},
				},
			},
		},
		"clientEventList": map[string]any{
			"clientEvent": []map[string]any{
				{
					"$": "br.com.sankhya.actionbutton.clientconfirm",
				},
			},
		},
	}

	// Executa usando o Cookie de sessão do usuário (igual ao execute-transaction)
	return c.ExecuteServiceWithCookie(ctx, "ActionButtonsSP.executeSTP", requestBody, snkSessionId)
}

func (c *Client) ConferirItem(ctx context.Context, input ConferirItemInput, snkSessionId string) (*TransactionResponse, error) {
	// Formata a data atual: DD/MM/YYYY HH:mm:00
	dataAtual := time.Now().Format("02/01/2006 15:04:00")

	// Montagem dos parâmetros dinâmicos
	params := []map[string]any{
		{
			"type":      "D",
			"paramName": "DTHCONF",
			"$":         dataAtual,
		},
		{
			"type":      "F", // "F" para Numérico (aceita int e float)
			"paramName": "QTDEMBARCADA",
			"$":         input.QtdEmbarcada,
		},
	}

	// Adiciona OBS apenas se foi preenchida (Opcional)
	if input.Obs != "" {
		params = append(params, map[string]any{
			"type":      "S",
			"paramName": "OBS",
			"$":         input.Obs,
		})
	}

	// Payload completo
	requestBody := map[string]any{
		"stpCall": map[string]any{
			"actionID":    "171",
			"procName":    "STP_CONFERIR_ITEM_ZNT",
			"rootEntity":  "AD_ZNTITEMCONF",
			"refreshType": "SEL",
			"params": map[string]any{
				"param": params,
			},
			"rows": map[string]any{
				"row": []map[string]any{
					// Mestre
					{
						"master":     "S",
						"entityName": "AD_ZNTCONFCAB",
						"field": []map[string]any{
							{
								"fieldName": "NUUNICO",
								"$":         fmt.Sprintf("%d", input.NuUnico),
							},
						},
					},
					// Detalhe
					{
						"field": []map[string]any{
							{
								"fieldName": "NUUNICO",
								"$":         fmt.Sprintf("%d", input.NuUnico),
							},
							{
								"fieldName": "NUMREG",
								"$":         fmt.Sprintf("%d", input.NumReg),
							},
						},
					},
				},
			},
		},
		"clientEventList": map[string]any{
			"clientEvent": []map[string]any{
				{
					"$": "br.com.sankhya.actionbutton.clientconfirm",
				},
			},
		},
	}

	return c.ExecuteServiceWithCookie(ctx, "ActionButtonsSP.executeSTP", requestBody, snkSessionId)
}

func (c *Client) FinalizarConferencia(ctx context.Context, input FinalizarConferenciaInput, snkSessionId string) (*TransactionResponse, error) {
	// Formata a data atual: DD/MM/YYYY HH:mm:00
	dataAtual := time.Now().Format("02/01/2006 15:04:00")

	// Montagem dos parâmetros dinâmicos
	params := []map[string]any{
		{
			"type":      "D",
			"paramName": "DTFIMCONF",
			"$":         dataAtual,
		},
	}

	// Adiciona OBSFIM apenas se foi preenchida (Opcional)
	if input.ObsFim != "" {
		params = append(params, map[string]any{
			"type":      "S",
			"paramName": "OBSFIM",
			"$":         input.ObsFim,
		})
	}

	// Payload completo
	requestBody := map[string]any{
		"stpCall": map[string]any{
			"actionID":    "173",
			"procName":    "STP_FINALIZAR_CONF_ZNT",
			"rootEntity":  "AD_ZNTCONFCAB",
			"refreshType": "SEL",
			"params": map[string]any{
				"param": params,
			},
			"rows": map[string]any{
				"row": []map[string]any{
					{
						"field": []map[string]any{
							{
								"fieldName": "NUUNICO",
								"$":         fmt.Sprintf("%d", input.NuUnico),
							},
						},
					},
				},
			},
		},
		"clientEventList": map[string]any{
			"clientEvent": []map[string]any{
				{
					"$": "br.com.sankhya.actionbutton.clientconfirm",
				},
			},
		},
	}

	return c.ExecuteServiceWithCookie(ctx, "ActionButtonsSP.executeSTP", requestBody, snkSessionId)
}