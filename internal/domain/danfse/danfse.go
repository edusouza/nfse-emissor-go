// Package danfse turns the XML of an authorised NFS-e into the strings the
// DANFSe prints.
//
// NT 008 v1.02 specifies the document field by field: where each one comes from
// in the NFS-e XML, how it is formatted, and what to print when the XML has
// nothing to say (a dash). Keeping that table inside one package means there is
// a single place to hold against the nota técnica when it changes; the renderer
// only puts the strings at the coordinates the same document gives.
//
// The values here carry accents, unlike the CLI's own messages: they are the
// content of a fiscal document, and the nota técnica spells them out.
package danfse

// Documento is a DANFSe ready to be laid out — every field already formatted
// and truncated as NT 008 prescribes.
type Documento struct {
	Cabecalho     Cabecalho
	Identificacao Identificacao
	Servico       Servico
	ISSQN         ISSQN
	Federal       Federal
	IBSCBS        IBSCBS
	Totais        Totais
}

// Cabecalho is the top strip of the document: item 2.4.3 of NT 008.
type Cabecalho struct {
	// Municipio is "Município: <nome> / <UF>", from xLocEmi and the emitter's
	// address. NT 008 leaves it out when the service code item is 99.
	Municipio string

	// AmbienteGerador says who generated the NFS-e (a city hall or the national
	// system), and TipoAmbiente whether it came from production or from the
	// restricted environment.
	AmbienteGerador string
	TipoAmbiente    string

	// SemValidadeJuridica reports tpAmb = 2. NT 008 requires the header of such
	// a document to carry "NFS-e SEM VALIDADE JURÍDICA" in red: a test invoice
	// that looks like a real one is the failure this prevents.
	SemValidadeJuridica bool

	// QRCode is the address the QR Code points to — the public lookup of this
	// access key at the national portal.
	QRCode string
}

// Identificacao is the "DADOS DA NFS-e" block: item 2.1.2 of NT 008.
type Identificacao struct {
	ChaveAcesso string
	Numero      string
	Competencia string
	EmissaoNFSe string
	NumeroDPS   string
	SerieDPS    string
	EmissaoDPS  string
	Emitente    string
	Situacao    string
	Finalidade  string
}

// Servico is the "SERVIÇO PRESTADO" block: item 2.1.7 of NT 008.
type Servico struct {
	CodigoTributacao string
	CodigoNBS        string
	LocalPrestacao   string

	// DescricaoCodigo is the municipal description when the invoice carries
	// one, and the national description otherwise — the rule the nota técnica
	// writes as "SE xTribMun <> '' ENTAO Descrição Municipal SENAO Nacional".
	// It is the one field with no label on the document.
	DescricaoCodigo string

	Descricao string
}

// ISSQN is the "TRIBUTAÇÃO MUNICIPAL" block: item 2.1.8 of NT 008.
type ISSQN struct {
	// Incide is false for an operation outside the municipal tax, and then the
	// block prints a single sentence instead of its fields — note 4.
	Incide bool

	TipoTributacao          string
	MunicipioIncidencia     string
	RegimeEspecial          string
	TipoImunidade           string
	SuspensaoExigibilidade  string
	NumeroProcessoSuspensao string
	BeneficioMunicipal      string
	CalculoBM               string
	TotalDeducoes           string
	DescontoIncondicionado  string
	BaseCalculo             string
	Aliquota                string
	Retencao                string
	Apurado                 string
}

// Federal is the "TRIBUTAÇÃO FEDERAL (EXCETO CBS)" block: item 2.1.9.
type Federal struct {
	IRRF                   string
	ContribuicaoPrevidenc  string
	ContribuicoesSociais   string
	PIS                    string
	COFINS                 string
	DescricaoContribuicoes string
}

// IBSCBS is the block of item 2.1.10, which only layout v1.01 fills. On a
// v1.00 invoice every field is a dash, and the block still prints: the nota
// técnica gives it no condition for being left out.
type IBSCBS struct {
	CST                string
	IndicadorOperacao  string
	ExclusoesReducoes  string
	BaseCalculo        string
	ReducaoAliquota    string
	AliquotaIBS        string
	AliquotaEfetivaMun string
	ValorApuradoMun    string
	AliquotaEfetivaUF  string
	ValorApuradoUF     string
	ValorTotalIBS      string
	AliquotaCBS        string
	AliquotaEfetivaCBS string
	ValorTotalCBS      string
}

// Totais is the "VALOR TOTAL DA NFS-e" block: item 2.1.11.
type Totais struct {
	ValorServico           string
	DescontoIncondicionado string
	DescontoCondicionado   string
	TotalRetencoes         string
	ValorLiquido           string
	TotalIBSCBS            string
	ValorLiquidoComIBSCBS  string
}
