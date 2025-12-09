package sankhya

import (
	"context"
	"fmt"
	"log/slog"
)

func (c *Client) GetUserPermissions(ctx context.Context, codUsu int) (*UserPermissions, error) {
	// A query permanece a mesma
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

	// --- FUNÇÕES AUXILIARES SEGURAS PARA EVITAR O PANIC ---
	
	// Converte interface{} para string de forma segura (trata nil)
	safeString := func(v any) string {
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}

	// Converte interface{} para int de forma segura (Sankhya retorna números como float64 em JSON)
	safeInt := func(v any) int {
		if v == nil {
			return 0
		}
		if f, ok := v.(float64); ok {
			return int(f)
		}
		// Tenta outras conversões se necessário
		return 0
	}

	// Converte "S"/"N" para bool de forma segura
	safeBool := func(v any) bool {
		if v == nil {
			return false
		}
		if s, ok := v.(string); ok {
			return s == "S"
		}
		return false
	}

	slog.Debug("Permissões carregadas", "codusu", codUsu)

	return &UserPermissions{
		ListaCodigos: safeString(row[0]),
		ListaNomes:   safeString(row[1]),
		CodUsu:       safeInt(row[2]),
		Transf:       safeBool(row[3]),
		Baixa:        safeBool(row[4]),
		Pick:         safeBool(row[5]),
		Corre:        safeBool(row[6]),
		BxaPick:      safeBool(row[7]),
		CriaPick:     safeBool(row[8]),
	}, nil
}