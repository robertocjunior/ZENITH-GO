package sankhya

// RomaneioDetalheInput recebe o número do fechamento
type RomaneioDetalheInput struct {
	NumeroFechamento int `json:"numero_fechamento"`
}

// RomaneioItem representa cada linha de produto do romaneio
type RomaneioItem struct {
	Tipo          string  `json:"tipo"`
	CodigoProduto string  `json:"codigo_produto"`
	Descricao     string  `json:"descricao"`
	Unidade       string  `json:"unidade"` // CODVOL
	Referencia    string  `json:"referencia"`
	CodigoBarras4 string  `json:"codigo_barras_4_digitos"`
	Quantidade    float64 `json:"quantidade"`
	PesoBruto     float64 `json:"peso_bruto"`
	Conferido     string  `json:"conferido"`
	NumReg        int     `json:"num_reg"`
	ListaBarras   string  `json:"lista_barras"`
}

// RomaneioDetalheResponse estrutura a resposta com cabeçalho único e lista de itens
type RomaneioDetalheResponse struct {
	Fechamento        int            `json:"fechamento"`
	NuUnico           int            `json:"nu_unico"`    // Novo Campo
	Data              string         `json:"data"`
	Motorista         string         `json:"motorista"`
	PesoTotal         float64        `json:"peso"`
	Placa             string         `json:"placa"`
	Veiculo           string         `json:"veiculo"`
	Paletes           float64        `json:"paletes"`
	CodUsuario        int            `json:"cod_usuario"`
	NomeUsuario       string         `json:"nome_usuario"`
	StatusConferencia string         `json:"status_conf"`
	Produtos          []RomaneioItem `json:"produtos"`
}