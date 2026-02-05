package sankhya

import (
	"context"
	"fmt"
)

// GetRomaneios executa a consulta de fechamentos de carga por data
func (c *Client) GetRomaneios(ctx context.Context, dataFiltro string) ([]RomaneioResult, error) {
	safeData := sanitizeStringForSql(dataFiltro)

	sql := fmt.Sprintf(`
		SELECT FEC.NUFECHAMENTO AS FECHAMENTO,
		       TO_CHAR(FEC.DTFECHAMENTO, 'DD/MM/YYYY') AS DATA,
		       PAR.NOMEPARC AS MOTORISTA,
		       VEI.PLACA AS PLACA,
		       VEI.AD_NUMINT || '-' || VEI.MARCAMODELO AS VEICULO,
		       VEI.AD_QTDPALLET AS PALETES
		  FROM AD_FECCAR FEC,
		       AD_FECMOT MOT,
		       TGFPAR PAR,
		       TGFROT ROT,
		       TGFVEI VEI
		 WHERE FEC.NUFECHAMENTO = MOT.NUFECHAMENTO
		   AND MOT.CODPARC = PAR.CODPARC
		   AND ROT.CODROTA = FEC.CODROTA
		   AND VEI.CODVEICULO = FEC.CODVEICULO
		   AND MOT.TIPO= 'M'
		   AND NVL(FEC.STATUS,'A') <> 'A'
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
			Placa:      getString(row[3]),
			Veiculo:    getString(row[4]),
			Paletes:    getFloat(row[5]),
		})
	}

	return results, nil
}