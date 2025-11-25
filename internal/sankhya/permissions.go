package sankhya

import "fmt"
import "context"

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
		return nil, fmt.Errorf("permissões não encontradas")
	}

	row := rows[0]
	toBool := func(val any) bool { return val.(string) == "S" }

	return &UserPermissions{
		ListaCodigos: fmt.Sprintf("%v", row[0]),
		ListaNomes:   fmt.Sprintf("%v", row[1]),
		CodUsu:       int(row[2].(float64)),
		Transf:       toBool(row[3]),
		Baixa:        toBool(row[4]),
		Pick:         toBool(row[5]),
		Corre:        toBool(row[6]),
		BxaPick:      toBool(row[7]),
		CriaPick:     toBool(row[8]),
	}, nil
}