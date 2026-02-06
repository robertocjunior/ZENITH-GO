package sankhya

import (
	"context"
	"fmt"
)

// GetRomaneios executa a consulta de fechamentos de carga com peso total
func (c *Client) GetRomaneios(ctx context.Context, dataFiltro string) ([]RomaneioResult, error) {
	safeData := sanitizeStringForSql(dataFiltro)

	sql := fmt.Sprintf(`
		SELECT FEC.NUFECHAMENTO AS FECHAMENTO,
		       TO_CHAR(FEC.DTFECHAMENTO, 'DD/MM/YYYY') AS DATA,
		       PAR.NOMEPARC AS MOTORISTA,
		       COM.PESO_TOTAL AS PESO,
		       VEI.PLACA AS PLACA,
		       VEI.AD_NUMINT || '-' || VEI.MARCAMODELO AS VEICULO,
		       VEI.AD_QTDPALLET AS PALETES
		  FROM AD_FECCAR FEC
		  JOIN AD_FECMOT MOT ON FEC.NUFECHAMENTO = MOT.NUFECHAMENTO
		  JOIN TGFPAR PAR ON MOT.CODPARC = PAR.CODPARC
		  JOIN TGFROT ROT ON FEC.CODROTA = ROT.CODROTA
		  JOIN TGFVEI VEI ON FEC.CODVEICULO = VEI.CODVEICULO
		  LEFT JOIN (
		        SELECT NUFECHAMENTO, 
		               ROUND(SUM(PESOBRUTO), 3) AS PESO_TOTAL
		          FROM AD_FECCOM
		         GROUP BY NUFECHAMENTO
		  ) COM ON FEC.NUFECHAMENTO = COM.NUFECHAMENTO
		 WHERE MOT.TIPO = 'M'
		   AND NVL(FEC.STATUS, 'A') <> 'A'
		   AND TRUNC(FEC.DTFECHAMENTO) = TO_DATE('%s', 'DD/MM/YYYY')
		 ORDER BY FEC.NUFECHAMENTO DESC`, safeData)

	rows, err := c.executeQuery(ctx, sql)
	if err != nil {
		return nil, err
	}

	var results []RomaneioResult
	for _, row := range rows {
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
			Fechamento: getInt(row[0]),
			Data:       getString(row[1]),
			Motorista:  getString(row[2]),
			Peso:       getFloat(row[3]),
			Placa:      getString(row[4]),
			Veiculo:    getString(row[5]),
			Paletes:    getFloat(row[6]),
		})
	}

	return results, nil
}

func (c *Client) GetRomaneioDetalhes(ctx context.Context, nuFec int) (*RomaneioDetalheResponse, error) {
	// Query Principal combinando Header (existente) + Seu Novo Select de Itens
	sql := fmt.Sprintf(`
SELECT 
    -- 0..6: Dados do Cabeçalho
    CABECALHO.FECHAMENTO, CABECALHO.DATA, CABECALHO.MOTORISTA, CABECALHO.PESO,
    CABECALHO.PLACA, CABECALHO.VEICULO, CABECALHO.PALETES,
    -- 7..14: Dados do Item (Seu novo SELECT)
    ITENS.TIPO,
    ITENS.CODPROD,
    ITENS.DESCRPROD,
    ITENS.CODVOL,
    ITENS.REFERENCIA,
    ITENS.CODBARRA4DIG,
    ITENS.QTDNEG,
    ITENS.PESOBRUTO
FROM (
    -- Subconsulta do Cabeçalho (Mantida igual)
    SELECT FEC.NUFECHAMENTO AS FECHAMENTO,
           TO_CHAR(FEC.DTFECHAMENTO, 'DD/MM/YYYY') AS DATA,
           PAR.NOMEPARC AS MOTORISTA,
           COM_PESO.PESO_TOTAL AS PESO,
           VEI.PLACA AS PLACA,
           VEI.AD_NUMINT || '-' || VEI.MARCAMODELO AS VEICULO,
           VEI.AD_QTDPALLET AS PALETES
      FROM AD_FECCAR FEC
      JOIN AD_FECMOT MOT ON FEC.NUFECHAMENTO = MOT.NUFECHAMENTO
      JOIN TGFPAR PAR ON MOT.CODPARC = PAR.CODPARC
      JOIN TGFVEI VEI ON FEC.CODVEICULO = VEI.CODVEICULO
      LEFT JOIN (
           SELECT NUFECHAMENTO, ROUND(SUM(PESOBRUTO), 3) AS PESO_TOTAL
             FROM AD_FECCOM GROUP BY NUFECHAMENTO
      ) COM_PESO ON FEC.NUFECHAMENTO = COM_PESO.NUFECHAMENTO
     WHERE MOT.TIPO = 'M' AND NVL(FEC.STATUS, 'A') <> 'A' AND FEC.NUFECHAMENTO = %d
) CABECALHO
CROSS JOIN (
    -- SEU NOVO SELECT DE ITENS AQUI
    SELECT DADOS.TIPO,
           DADOS.CODPROD,
           DADOS.DESCRPROD || CASE WHEN DADOS.CONTROLE = ' ' THEN '' ELSE ' - '||DADOS.CONTROLE END || ' ' || DADOS.MARCA || ' - ' || NVL(DADOS.DESCRDANFE, ' ') AS DESCRPROD,
           DADOS.CODVOL,
           DADOS.REFERENCIA,
           DADOS.CODBARRA4DIG,
           DADOS.QTDNEG,
           DADOS.PESOBRUTO
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
               SUM((CASE WHEN VOA.CODPROD IS NULL THEN ITE.QTDNEG
                         WHEN VOA.DIVIDEMULTIPLICA = 'D' THEN ITE.QTDNEG * VOA.QUANTIDADE
                         ELSE ITE.QTDNEG / VOA.QUANTIDADE END)) AS QTDNEG,
               SUM(CASE WHEN (EXISTS(SELECT 1 FROM TGFVAR VAR WHERE VAR.NUNOTA = ITE.NUNOTA AND VAR.NUNOTAORIG = VAR.NUNOTA AND VAR.SEQUENCIAORIG = ITE.SEQUENCIA))
                        THEN (SELECT SUM(ITE2.QTDNEG * PRO2.PESOBRUTO) FROM TGFITE ITE2, TGFPRO PRO2, TGFVAR VAR2 WHERE ITE2.CODPROD = PRO2.CODPROD AND ITE2.NUNOTA = CAB.NUNOTA AND ITE2.USOPROD = 'D' AND VAR2.NUNOTA = ITE.NUNOTA AND VAR2.SEQUENCIAORIG = ITE.SEQUENCIA AND VAR2.SEQUENCIA = ITE2.SEQUENCIA)
                        ELSE ITE.QTDNEG * PRO.PESOBRUTO END) AS PESOBRUTO
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
         GROUP BY CASE WHEN CAB.AD_AGRUPROM IS NOT NULL THEN ' ' ||TO_CHAR(CAB.AD_AGRUPROM) ELSE COM.TIPO END,
                  TO_CHAR(ITE.CODPROD), ITE.CODVOL, SUBSTR(BAR.CODBARRA, -4), ITE.CONTROLE, PRO.DESCRPROD, PRO.MARCA, VOA.DESCRDANFE, PRO.REFERENCIA
        UNION
        SELECT COM.TIPO,
               NVL(TO_CHAR(PRO.CODPROD),'NFe ' || TO_CHAR(NOTA.NUMNOTA)) AS CODPROD,
               NVL(VOA.CODVOL,ITE.CODVOL) AS CODVOL,
               ' ' AS CONTROLE,
               NVL(PRO.DESCRPROD,ITE.DESCRPROD) AS DESCRPROD,
               NVL(PRO.MARCA,' ') AS MARCA,
               NVL(VOA.DESCRDANFE,' ') AS DESCRDANFE,
               NVL(PRO.REFERENCIA,' ') AS REFERENCIA,
               ' ' AS CODBARRA4DIG,
               SUM(ITE.QTDNEG) AS QTDNEG,
               SUM((NOTA.PESOTOT / NOTA.QTDVOL) * ITE.QTDNEG) AS PESOBRUTO
          FROM TMSNOTAS NOTA
         INNER JOIN TMSNOTASITE ITE ON NOTA.NROUNICO = ITE.NROUNICO
         INNER JOIN AD_FECCOM COM ON NOTA.NUNOTACTE = COM.NUMDOCUMENTO
         INNER JOIN AD_FECCAR FEC ON FEC.NUFECHAMENTO = COM.NUFECHAMENTO
          LEFT JOIN TGFPAP PAP ON PAP.CODPROPARC = ITE.CODPROPARC AND NOTA.CODPARCREM = PAP.CODPARC AND ITE.CODVOL = PAP.UNIDADE AND PAP.SEQUENCIA = (SELECT MAX(SEQUENCIA) FROM TGFPAP WHERE NOTA.CODPARCREM = CODPARC AND ITE.CODVOL = UNIDADE AND CODPROPARC = ITE.CODPROPARC)
          LEFT JOIN TGFPRO PRO ON PAP.CODPROD = PRO.CODPROD
          LEFT JOIN TGFVOA VOA ON PRO.CODPROD = VOA.CODPROD AND ITE.CODVOL = VOA.CODVOL
         WHERE COM.TIPO = 'C'
           AND FEC.NUFECHAMENTO = %d
         GROUP BY COM.TIPO, NVL(TO_CHAR(PRO.CODPROD),'NFe ' || TO_CHAR(NOTA.NUMNOTA)), NVL(VOA.CODVOL,ITE.CODVOL), ' ', NVL(PRO.DESCRPROD,ITE.DESCRPROD), NVL(PRO.MARCA,' '), NVL(VOA.DESCRDANFE,' '), NVL(PRO.REFERENCIA,' '), ' '
      ) DADOS
    ORDER BY 1 DESC, 2, 3, 4
) ITENS`, nuFec, nuFec, nuFec)

	rows, err := c.executeQuery(ctx, sql)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("nenhum registro encontrado para o fechamento %d", nuFec)
	}

	// Helpers para evitar PANIC com valores NULL
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

	// Mapeia o cabeçalho (0-6)
	res := &RomaneioDetalheResponse{
		Fechamento: getInt(rows[0][0]),
		Data:       getString(rows[0][1]),
		Motorista:  getString(rows[0][2]),
		PesoTotal:  getFloat(rows[0][3]),
		Placa:      getString(rows[0][4]),
		Veiculo:    getString(rows[0][5]),
		Paletes:    getFloat(rows[0][6]),
		Produtos:   []RomaneioItem{},
	}

	// Mapeia os itens (7-14)
	for _, row := range rows {
		res.Produtos = append(res.Produtos, RomaneioItem{
			Tipo:          getString(row[7]),
			CodigoProduto: getString(row[8]),
			Descricao:     getString(row[9]),  // Novo campo concatenado
			Unidade:       getString(row[10]), // CODVOL
			Referencia:    getString(row[11]),
			CodigoBarras4: getString(row[12]),
			Quantidade:    getFloat(row[13]),
			PesoBruto:     getFloat(row[14]),
		})
	}

	return res, nil
}