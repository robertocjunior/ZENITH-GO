package sankhya

import "errors"

// --- Erros Públicos ---
var (
	ErrUserNotFound          = errors.New("usuário inexistente ou nome incorreto")
	ErrUserNotAuthorized     = errors.New("usuário não possui autorização de acesso (AD_APPPERM)")
	ErrDevicePendingApproval = errors.New("dispositivo não autorizado. Solicite a liberação ao administrador")
	ErrItemNotFound          = errors.New("item não encontrado")
	ErrPermissionDenied      = errors.New("permissão negada para esta operação")
)

// --- Structs de Login (Service Account & Mobile) ---

type loginResponse struct {
	BearerToken string `json:"bearerToken"`
	Error       any    `json:"error"`
}

type simpleValue struct {
	Value string `json:"$"`
}

type mobileLoginRequest struct {
	ServiceName string `json:"serviceName"`
	RequestBody struct {
		NomUsu        simpleValue `json:"NOMUSU"`
		Interno       simpleValue `json:"INTERNO"`
		KeepConnected simpleValue `json:"KEEPCONNECTED"`
	} `json:"requestBody"`
}

type mobileLoginResponse struct {
	Status       string `json:"status"`
	ResponseBody struct {
		JSessionID simpleValue `json:"jsessionid"`
	} `json:"responseBody"`
}

// --- Structs de Execução de SQL (DbExplorerSP) ---

type dbExplorerRequest struct {
	ServiceName string `json:"serviceName"`
	RequestBody struct {
		SQL    string `json:"sql"`
		Params map[string]any `json:"params"`
	} `json:"requestBody"`
}

type dbExplorerResponse struct {
	Status       string `json:"status"`
	ResponseBody struct {
		Rows [][]any `json:"rows"`
	} `json:"responseBody"`
}

// --- Structs de Dataset (Legado/Interno) ---
// Usado por métodos internos como registerDevice

type datasetSaveRequest struct {
	ServiceName string `json:"serviceName"`
	RequestBody struct {
		EntityName string `json:"entityName"`
		Fields     []string `json:"fields"`
		Records    []datasetRecord `json:"records"`
	} `json:"requestBody"`
}

type datasetRecord struct {
	Values map[string]string `json:"values"`
}

type datasetSaveResponse struct {
	Status string `json:"status"`
}

// --- Structs para Serviço de Transações (Novos) ---
// Usados pelo TransactionHandler e TransactionService

type TransactionResponse struct {
	ServiceName   string `json:"serviceName"`
	Status        string `json:"status"`
	StatusMessage string `json:"statusMessage"` // Ocasionalmente retornado em erros
	ResponseBody  struct {
		Result [][]string `json:"result"` // Para DatasetSP.save
	} `json:"responseBody"`
}

// Estrutura genérica para chamadas de serviço
type ServiceRequest struct {
	ServiceName string `json:"serviceName"`
	RequestBody any    `json:"requestBody"`
}

// Payload flexível para DatasetSP.save (Suporta PK e StandAlone)
type DatasetSaveBody struct {
	EntityName string          `json:"entityName"`
	Fields     []string        `json:"fields"`
	Records    []DatasetRecord `json:"records"`
	StandAlone bool            `json:"standAlone,omitempty"`
}

type DatasetRecord struct {
	PK     map[string]string `json:"pk,omitempty"`
	Values map[string]string `json:"values"`
}

// Payload para ActionButtonsSP.executeScript
type ExecuteScriptBody struct {
	RunScript struct {
		ActionID    string `json:"actionID"`
		RefreshType string `json:"refreshType"`
		Params      struct {
			Param []ScriptParam `json:"param"`
		} `json:"params"`
		Rows struct {
			Row []ScriptRow `json:"row"`
		} `json:"rows"`
	} `json:"runScript"`
	ClientEventList struct {
		ClientEvent []map[string]string `json:"clientEvent"`
	} `json:"clientEventList"`
}

type ScriptParam struct {
	Type      string `json:"type"`
	ParamName string `json:"paramName"`
	Value     any    `json:"$"`
}

type ScriptRow struct {
	Field []ScriptField `json:"field"`
}

type ScriptField struct {
	FieldName string `json:"fieldName"`
	Value     string `json:"$"`
}

// Payload para ActionButtonsSP.executeSTP
type ExecuteSTPBody struct {
	StpCall struct {
		ActionID   string `json:"actionID"`
		ProcName   string `json:"procName"`
		RootEntity string `json:"rootEntity"`
		Rows       struct {
			Row []ScriptRow `json:"row"`
		} `json:"rows"`
	} `json:"stpCall"`
}

// --- Structs de Domínio (Retornos para o Frontend) ---

type UserPermissions struct {
	CodUsu       int    `json:"CODUSU"`
	ListaCodigos string `json:"LISTA_CODIGOS"`
	ListaNomes   string `json:"LISTA_NOMES"`
	Transf       bool   `json:"TRANSF"`
	Baixa        bool   `json:"BAIXA"`
	Pick         bool   `json:"PICK"`
	Corre        bool   `json:"CORRE"`
	BxaPick      bool   `json:"BXAPICK"`
	CriaPick     bool   `json:"CRIAPICK"`
}

type ItemDetail struct {
	CodArm      int     `json:"codArm"`
	SeqEnd      int     `json:"seqEnd"`
	CodRua      string  `json:"codRua"`
	CodPrd      int     `json:"codPrd"`
	CodApt      string  `json:"codApt"`
	CodProd     int     `json:"codProd"`
	DescrProd   string  `json:"descrProd"`
	Marca       string  `json:"marca"`
	DatVal      string  `json:"datVal"`
	QtdPro      float64 `json:"qtdPro"`
	EndPic      string  `json:"endPic"`
	NumDoc      int     `json:"numDoc"`
	QtdCompleta string  `json:"qtdCompleta"`
	Derivacao   string  `json:"derivacao"`
}

type SearchItemResult struct {
	SeqEnd      int     `json:"seqEnd"`
	CodRua      string  `json:"codRua"`
	CodPrd      int     `json:"codPrd"`
	CodApt      string  `json:"codApt"`
	CodProd     int     `json:"codProd"`
	DescrProd   string  `json:"descrProd"`
	Marca       string  `json:"marca"`
	DatVal      string  `json:"datVal"`
	QtdPro      float64 `json:"qtdPro"`
	EndPic      string  `json:"endPic"`
	QtdCompleta string  `json:"qtdCompleta"`
	Derivacao   string  `json:"derivacao"`
}

type PickingLocation struct {
	SeqEnd    int    `json:"seqEnd"`
	DescrProd string `json:"descrProd"`
}

type HistoryItem struct {
	Tipo       string  `json:"tipo"`
	DatGer     string  `json:"datGer"`
	Hora       string  `json:"hora"`
	CodArm     int     `json:"codArm"`
	SeqEnd     int     `json:"seqEnd"`
	ArmDes     string  `json:"armDes"`
	EndDes     string  `json:"endDes"`
	CodProd    int     `json:"codProd"`
	DescrProd  string  `json:"descrProd"`
	Marca      string  `json:"marca"`
	Derivacao  string  `json:"derivacao"`
	QuantAnt   float64 `json:"quantAnt"`
	QtdAtual   float64 `json:"qtdAtual"`
	IdOperacao int     `json:"idOperacao"`
	SeqIte     int     `json:"seqIte"`
}