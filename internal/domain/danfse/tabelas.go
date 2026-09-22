package danfse

// The XML carries codes; NT 008 says to print the descriptions ("Utilizar a
// descrição destas opções"). The descriptions below are the ones the schema
// documents for each type, so the two artefacts can be compared side by side
// when either changes.

// ambienteGerador describes TSAmbGeradorNFSe (infNFSe/ambGer).
var ambienteGerador = map[string]string{
	"1": "Prefeitura",
	"2": "Sistema Nacional da NFS-e",
}

// tipoAmbiente describes TSTipoAmbiente (infDPS/tpAmb).
var tipoAmbiente = map[string]string{
	"1": "Produção",
	"2": "Homologação",
}

// emitenteDPS describes TSEmitenteDPS (infDPS/tpEmit).
var emitenteDPS = map[string]string{
	"1": "Prestador",
	"2": "Tomador",
	"3": "Intermediário",
}

// situacaoNFSe describes TStat (infNFSe/cStat).
var situacaoNFSe = map[string]string{
	"100": "NFS-e Gerada",
	"102": "NFS-e de Decisão Judicial",
	"103": "NFS-e Avulsa",
	"107": "NFS-e MEI",
}

// finalidadeNFSe describes TSRTCFinNFSe (infDPS/IBSCBS/finNFSe), which only
// exists in layout v1.01. An invoice issued under v1.00 has no such element,
// and the field prints as a dash — note 12 of NT 008.
var finalidadeNFSe = map[string]string{
	"0": "NFS-e regular",
}

// descrever returns the description of a code, or the code itself when the
// table does not know it.
//
// Printing the raw code is deliberate: a code the government added after this
// binary was built still belongs on the document, and an empty field would hide
// that the invoice says something this program does not understand.
func descrever(tabela map[string]string, codigo string) string {
	if codigo == "" {
		return ""
	}
	if descricao, ok := tabela[codigo]; ok {
		return descricao
	}
	return codigo
}
