package sankhya

// RomaneioInput representa o filtro de data enviado pelo body
type RomaneioInput struct {
	Data string `json:"data"` // Formato DD/MM/YYYY
}

// RomaneioResult representa uma linha do select solicitado
type RomaneioResult struct {
	Fechamento int     `json:"fechamento"`
	Data       string  `json:"data"`
	Motorista  string  `json:"motorista"`
	Placa      string  `json:"placa"`
	Veiculo    string  `json:"veiculo"`
	Paletes    float64 `json:"paletes"`
}