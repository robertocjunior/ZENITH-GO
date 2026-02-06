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