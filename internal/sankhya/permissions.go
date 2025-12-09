package sankhya

import (
	"context"
	"fmt"
	"log/slog"
)

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
		slog.Warn("Usuário sem configuração de permissões", "codusu", codUsu)
		return nil, fmt.Errorf("permissões não encontradas")
	}

	row := rows[0]

	// Helper seguro para converter "S"/"N" em bool, tratando NULOS
	toBool := func(val any) bool {
		if val == nil {
			return false
		}
		if str, ok := val.(string); ok {
			return str == "S"
		}
		return false
	}

	// Helper seguro para Inteiros (Sankhya geralmente retorna números como float64 no driver JSON)
	toInt := func(val any) int {
		if val == nil {
			return 0
		}
		if f, ok := val.(float64); ok {
			return int(f)
		}
		return 0
	}

	// Helper seguro para Strings
	toString := func(val any) string {
		if val == nil {
			return ""
		}
		return fmt.Sprintf("%v", val)
	}

	slog.Debug("Permissões carregadas", "codusu", codUsu)

	return &UserPermissions{
		ListaCodigos: toString(row[0]),
		ListaNomes:   toString(row[1]),
		CodUsu:       toInt(row[2]),
		Transf:       toBool(row[3]),
		Baixa:        toBool(row[4]),
		Pick:         toBool(row[5]),
		Corre:        toBool(row[6]),
		BxaPick:      toBool(row[7]),
		CriaPick:     toBool(row[8]),
	}, nil
}