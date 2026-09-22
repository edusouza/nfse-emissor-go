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
