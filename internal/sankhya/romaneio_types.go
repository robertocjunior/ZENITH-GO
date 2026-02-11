package sankhya

// RomaneioInput representa o filtro de data enviado pelo body
type RomaneioInput struct {
	Data string `json:"data"` // Formato DD/MM/YYYY
}

// RomaneioResult representa uma linha do select atualizado
type RomaneioResult struct {
	Fechamento  int     `json:"fechamento"`
	Data        string  `json:"data"`
	Motorista   string  `json:"motorista"`
	Peso        float64 `json:"peso"`
	Placa       string  `json:"placa"`
	Veiculo     string  `json:"veiculo"`
	Paletes     float64 `json:"paletes"`
	CodUsuario  int     `json:"cod_usuario"`
	NomeUsuario string  `json:"nome_usuario"`
	Status      string  `json:"status"`
}

type IniciarConferenciaInput struct {
	NuUnico int `json:"nu_unico"`
}

type ConferirItemInput struct {
	NuUnico      int     `json:"nu_unico"`
	NumReg       int     `json:"num_reg"`
	QtdEmbarcada float64 `json:"qtd_embarcada"`
	Obs          string  `json:"obs"`
}