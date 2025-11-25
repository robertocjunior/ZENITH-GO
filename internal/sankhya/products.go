package sankhya

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
)

// GetItemDetails busca detalhes de um item específico
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
func (c *Client) GetPickingLocations(ctx context.Context, codArm int, codProd int, sequenciaExclude int) (map[string]PickingLocation, error) {
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

	results := make(map[string]PickingLocation)
	for _, row := range rows {
		getInt := func(i int) int {
			if i >= len(row) || row[i] == nil { return 0 }
			if f, ok := row[i].(float64); ok { return int(f) }
			return 0
		}
		getString := func(i int) string {
			if i >= len(row) || row[i] == nil { return "" }
			return fmt.Sprintf("%v", row[i])
		}

		seq := getInt(0)
		strSeq := strconv.Itoa(seq)

		results[strSeq] = PickingLocation{
			SeqEnd:    seq,
			DescrProd: getString(1),
		}
	}
	
	slog.Debug("Locais de picking encontrados", "count", len(results))
	return results, nil
}

// SearchItems busca itens no armazém
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

// GetHistory busca o histórico de movimentação
func (c *Client) GetHistory(ctx context.Context, dtIni string, dtFim string, codUsu int) ([]HistoryItem, error) {
	safeDtIni := sanitizeStringForSql(dtIni)
	safeDtFim := sanitizeStringForSql(dtFim)
	
	codUsuStr := "NULL"
	if codUsu > 0 {
		codUsuStr = strconv.Itoa(codUsu)
	}

	sql := fmt.Sprintf(`
		SELECT 'MOV' AS TIPO, 
		       BXA.DATGER, 
		       TO_CHAR(BXA.DATGER, 'HH24:MI:SS') AS HORA, 
		       IBX.CODARM, 
		       IBX.SEQEND, 
		       IBX.ARMDES, 
		       IBX.ENDDES, 
		       IBX.CODPROD, 
		       PRO.DESCRPROD, 
		       PRO.MARCA, 
		       (SELECT MAX(V.DESCRDANFE) 
		        FROM TGFVOA V 
		        WHERE V.CODPROD = IBX.CODPROD 
		          AND V.CODVOL = PRO.CODVOL) AS DERIVACAO, 
		       NULL AS QUANT_ANT, 
		       NULL AS QTD_ATUAL, 
		       BXA.SEQBAI AS ID_OPERACAO, 
		       IBX.SEQITE
		FROM AD_BXAEND BXA 
		JOIN AD_IBXEND IBX ON IBX.SEQBAI = BXA.SEQBAI 
		LEFT JOIN TGFPRO PRO ON IBX.CODPROD = PRO.CODPROD
		WHERE (BXA.USUGER = %s OR %s IS NULL)
		  AND TRUNC(BXA.DATGER) BETWEEN TO_DATE('%s', 'DD/MM/YYYY') AND TO_DATE('%s', 'DD/MM/YYYY')

		UNION ALL

		SELECT 'CORRECAO' AS TIPO, 
		       H.DTHOPER, 
		       TO_CHAR(H.DTHOPER, 'HH24:MI:SS') AS HORA, 
		       H.CODARM, 
		       H.SEQEND, 
		       NULL, 
		       NULL, 
		       H.CODPROD, 
		       (SELECT P.DESCRPROD FROM TGFPRO P WHERE P.CODPROD = H.CODPROD), 
		       H.MARCA, 
		       H.DERIV, 
		       H.QUANT, 
		       H.QATUAL, 
		       H.NUMUNICO, 
		       NULL
		FROM AD_HISTENDAPP H
		WHERE (H.CODUSU = %s OR %s IS NULL)
		  AND TRUNC(H.DTHOPER) BETWEEN TO_DATE('%s', 'DD/MM/YYYY') AND TO_DATE('%s', 'DD/MM/YYYY')

		ORDER BY 2 DESC, 15 ASC`, 
		codUsuStr, codUsuStr, safeDtIni, safeDtFim, 
		codUsuStr, codUsuStr, safeDtIni, safeDtFim)

	rows, err := c.executeQuery(ctx, sql)
	if err != nil {
		return nil, err
	}

	var results []HistoryItem
	for _, row := range rows {
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

		results = append(results, HistoryItem{
			Tipo:       getString(0),
			DatGer:     getString(1),
			Hora:       getString(2),
			CodArm:     getInt(3),
			SeqEnd:     getInt(4),
			ArmDes:     getString(5),
			EndDes:     getString(6),
			CodProd:    getInt(7),
			DescrProd:  getString(8),
			Marca:      getString(9),
			Derivacao:  getString(10),
			QuantAnt:   getFloat(11),
			QtdAtual:   getFloat(12),
			IdOperacao: getInt(13),
			SeqIte:     getInt(14),
		})
	}
	
	slog.Debug("Histórico retornado", "count", len(results))
	return results, nil
}