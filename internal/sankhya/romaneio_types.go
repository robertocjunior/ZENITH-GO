package sankhya

// RomaneioInput representa o filtro de data enviado pelo body
type RomaneioInput struct {
	Data string `json:"data"` // Formato DD/MM/YYYY
}

// RomaneioResult representa uma linha do select atualizado
type RomaneioResult struct {
	Fechamento int     `json:"fechamento"`
	Data       string  `json:"data"`
	Motorista  string  `json:"motorista"`
	Peso       float64 `json:"peso"`
	Placa      string  `json:"placa"`
	Veiculo    string  `json:"veiculo"`
	Paletes    float64 `json:"paletes"`
}