package sankhya

import (
	"context"
	"fmt"
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
    -- 0..10: Dados do Cabeçalho
    CABECALHO.FECHAMENTO, 
    CABECALHO.NUUNICO, -- Novo campo (Índice 1)
    CABECALHO.DATA, 
    CABECALHO.MOTORISTA, 
    CABECALHO.PESO,
    CABECALHO.PLACA, 
    CABECALHO.VEICULO, 
    CABECALHO.PALETES,
    CABECALHO.CODUSU,
    CABECALHO.NOMEUSU,
    CABECALHO.STATUS_CONF,
    -- 11..19: Dados do Item
    ITENS.TIPO,
    ITENS.CODPROD,
    ITENS.DESCRPROD,
    ITENS.CODVOL,
    ITENS.REFERENCIA,
    ITENS.CODBARRA4DIG,
    ITENS.QTDNEG,
    ITENS.PESOBRUTO,
    NVL(CONF.CONFERIDO, 'N') AS CONFERIDO
FROM (
    -- Subconsulta do Cabeçalho
    SELECT FEC.NUFECHAMENTO AS FECHAMENTO,
           FCAB.NUUNICO, -- Selecionado da tabela AD_ZNTCONFCAB
           TO_CHAR(FEC.DTFECHAMENTO, 'DD/MM/YYYY') AS DATA,
           PAR.NOMEPARC AS MOTORISTA,
           COM_PESO.PESO_TOTAL AS PESO,
           VEI.PLACA AS PLACA,
           VEI.AD_NUMINT || '-' || VEI.MARCAMODELO AS VEICULO,
           VEI.AD_QTDPALLET AS PALETES,
           FCAB.CODUSU,
           USU.NOMEUSUCPLT AS NOMEUSU,
           FCAB.STATUS AS STATUS_CONF
      FROM AD_FECCAR FEC
      JOIN AD_FECMOT MOT ON FEC.NUFECHAMENTO = MOT.NUFECHAMENTO
      JOIN TGFPAR PAR ON MOT.CODPARC = PAR.CODPARC
      JOIN TGFVEI VEI ON FEC.CODVEICULO = VEI.CODVEICULO
      JOIN AD_ZNTCONFCAB FCAB ON FCAB.NUFECHAMENTO = FEC.NUFECHAMENTO
      LEFT JOIN TSIUSU USU ON USU.CODUSU = FCAB.CODUSU
      LEFT JOIN (
            SELECT NUFECHAMENTO, ROUND(SUM(PESOBRUTO), 3) AS PESO_TOTAL
              FROM AD_FECCOM GROUP BY NUFECHAMENTO
      ) COM_PESO ON FEC.NUFECHAMENTO = COM_PESO.NUFECHAMENTO
     WHERE MOT.TIPO = 'M' 
       AND NVL(FEC.STATUS, 'A') <> 'A' 
       AND FEC.NUFECHAMENTO = %d
) CABECALHO
CROSS JOIN (
    -- SELECT DE ITENS
    SELECT DADOS.TIPO,
           DADOS.CODPROD,
           DADOS.DESCRPROD || CASE WHEN DADOS.CONTROLE = ' ' THEN '' ELSE ' - '||DADOS.CONTROLE END || ' ' || DADOS.MARCA || ' - ' || NVL(DADOS.DESCRDANFE, ' ') AS DESCRPROD,
           DADOS.CODVOL,
           DADOS.REFERENCIA,
           DADOS.CODBARRA4DIG,
           DADOS.QTDNEG,
           DADOS.PESOBRUTO,
           DADOS.NUFECHAMENTO
      FROM (
        SELECT CASE WHEN CAB.AD_AGRUPROM IS NOT NULL THEN ' ' ||TO_CHAR(CAB.AD_AGRUPROM) ELSE COM.TIPO END AS TIPO,
               TO_CHAR(ITE.CODPROD) AS CODPROD,
               ITE.CODVOL,
               NVL(ITE.CONTROLE,' ') AS CONTROLE,
               PRO.DESCRPROD,
               PRO.MARCA,
               VOA.DESCRDANFE,
               PRO.REFERENCIA,
               SUBSTR(BAR.CODBARRA, -4) AS CODBARRA4DIG,
               ROUND(SUM((CASE WHEN VOA.CODPROD IS NULL THEN ITE.QTDNEG
                         WHEN VOA.DIVIDEMULTIPLICA = 'D' THEN ITE.QTDNEG * VOA.QUANTIDADE
                         ELSE ITE.QTDNEG / VOA.QUANTIDADE END)), 3) AS QTDNEG,
               ROUND(SUM(CASE WHEN (EXISTS(SELECT 1 FROM TGFVAR VAR WHERE VAR.NUNOTA = ITE.NUNOTA AND VAR.NUNOTAORIG = VAR.NUNOTA AND VAR.SEQUENCIAORIG = ITE.SEQUENCIA))
                        THEN (SELECT SUM(ITE2.QTDNEG * PRO2.PESOBRUTO) FROM TGFITE ITE2, TGFPRO PRO2, TGFVAR VAR2 WHERE ITE2.CODPROD = PRO2.CODPROD AND ITE2.NUNOTA = CAB.NUNOTA AND ITE2.USOPROD = 'D' AND VAR2.NUNOTA = ITE.NUNOTA AND VAR2.SEQUENCIAORIG = ITE.SEQUENCIA AND VAR2.SEQUENCIA = ITE2.SEQUENCIA)
                        ELSE ITE.QTDNEG * PRO.PESOBRUTO END), 3) AS PESOBRUTO,
               FEC.NUFECHAMENTO
          FROM AD_FECCAR FEC, AD_FECCOM COM, TGFCAB CAB, TGFPRO PRO, TGFITE ITE
          LEFT JOIN TGFVOA VOA ON (VOA.CODPROD = ITE.CODPROD AND VOA.CODVOL = ITE.CODVOL AND ((ITE.CONTROLE IS NULL AND VOA.CONTROLE = ' ') OR (ITE.CONTROLE IS NOT NULL AND ITE.CONTROLE = VOA.CONTROLE)))
          LEFT JOIN TGFVOA BAR ON (BAR.CODPROD = ITE.CODPROD AND BAR.UNIDTRIB = 'S' AND ((ITE.CONTROLE IS NULL AND BAR.CONTROLE = ' ') OR (ITE.CONTROLE IS NOT NULL AND ITE.CONTROLE = BAR.CONTROLE)))
         WHERE FEC.NUFECHAMENTO = COM.NUFECHAMENTO
           AND COM.CODEMP = (SELECT CASE WHEN EMP.CODEMPOC IS NULL THEN EMP.CODEMP ELSE CODEMPOC END FROM TGFEMP EMP WHERE EMP.CODEMP = CAB.CODEMP)
           AND CAB.ORDEMCARGA = COM.NUMDOCUMENTO
           AND ITE.NUNOTA = CAB.NUNOTA
           AND ITE.CODPROD = PRO.CODPROD
           AND CAB.TIPMOV = 'P'
           AND COM.TIPO = 'O'
           AND ITE.USOPROD <> 'D'
           AND NVL(CAB.STATUSNFE, 'X') <> 'D'
           AND FEC.NUFECHAMENTO = %d
         GROUP BY FEC.NUFECHAMENTO, CASE WHEN CAB.AD_AGRUPROM IS NOT NULL THEN ' ' ||TO_CHAR(CAB.AD_AGRUPROM) ELSE COM.TIPO END,
                  TO_CHAR(ITE.CODPROD), ITE.CODVOL, SUBSTR(BAR.CODBARRA, -4), ITE.CONTROLE, PRO.DESCRPROD, PRO.MARCA, VOA.DESCRDANFE, PRO.REFERENCIA
        UNION ALL
        SELECT COM.TIPO,
               NVL(TO_CHAR(PRO.CODPROD),'NFe ' || TO_CHAR(NOTA.NUMNOTA)) AS CODPROD,
               NVL(VOA.CODVOL,ITE.CODVOL) AS CODVOL,
               ' ' AS CONTROLE,
               NVL(PRO.DESCRPROD,ITE.DESCRPROD) AS DESCRPROD,
               NVL(PRO.MARCA,' ') AS MARCA,
               NVL(VOA.DESCRDANFE,' ') AS DESCRDANFE,
               NVL(PRO.REFERENCIA,' ') AS REFERENCIA,
               ' ' AS CODBARRA4DIG,
               ROUND(SUM(ITE.QTDNEG), 3) AS QTDNEG,
               ROUND(SUM((NOTA.PESOTOT / NOTA.QTDVOL) * ITE.QTDNEG), 3) AS PESOBRUTO,
               FEC.NUFECHAMENTO
          FROM TMSNOTAS NOTA
         INNER JOIN TMSNOTASITE ITE ON NOTA.NROUNICO = ITE.NROUNICO
         INNER JOIN AD_FECCOM COM ON NOTA.NUNOTACTE = COM.NUMDOCUMENTO
         INNER JOIN AD_FECCAR FEC ON FEC.NUFECHAMENTO = COM.NUFECHAMENTO
          LEFT JOIN TGFPAP PAP ON PAP.CODPROPARC = ITE.CODPROPARC AND NOTA.CODPARCREM = PAP.CODPARC AND ITE.CODVOL = PAP.UNIDADE AND PAP.SEQUENCIA = (SELECT MAX(SEQUENCIA) FROM TGFPAP WHERE NOTA.CODPARCREM = CODPARC AND ITE.CODVOL = UNIDADE AND CODPROPARC = ITE.CODPROPARC)
          LEFT JOIN TGFPRO PRO ON PAP.CODPROD = PRO.CODPROD
          LEFT JOIN TGFVOA VOA ON PRO.CODPROD = VOA.CODPROD AND ITE.CODVOL = VOA.CODVOL
         WHERE COM.TIPO = 'C'
           AND FEC.NUFECHAMENTO = %d
         GROUP BY FEC.NUFECHAMENTO, COM.TIPO, NVL(TO_CHAR(PRO.CODPROD),'NFe ' || TO_CHAR(NOTA.NUMNOTA)), NVL(VOA.CODVOL,ITE.CODVOL), ' ', NVL(PRO.DESCRPROD,ITE.DESCRPROD), NVL(PRO.MARCA,' '), NVL(VOA.DESCRDANFE,' '), NVL(PRO.REFERENCIA,' '), ' '
      ) DADOS
) ITENS
LEFT JOIN AD_ZNTITEMCONF CONF 
    ON  ITENS.NUFECHAMENTO = CONF.NUFECHAMENTO 
    AND ITENS.CODPROD      = TO_CHAR(CONF.CODPROD) 
    AND ITENS.CODVOL       = CONF.CODVOL
ORDER BY ITENS.TIPO DESC, ITENS.CODPROD, ITENS.CODVOL`, nuFec, nuFec, nuFec)

	rows, err := c.executeQuery(ctx, sql)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("nenhum registro encontrado para o fechamento %d", nuFec)
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

	// Mapeia o cabeçalho (0-10) - Indices atualizados
	res := &RomaneioDetalheResponse{
		Fechamento:        getInt(rows[0][0]),
		NuUnico:           getInt(rows[0][1]), // Novo Campo
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

	// Mapeia os itens (11-19) - Indices atualizados
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
		})
	}

	return res, nil
}