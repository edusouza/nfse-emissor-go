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

// tributacaoISSQN describes TSTribISSQN (valores/trib/tribMun/tribISSQN).
var tributacaoISSQN = map[string]string{
	"1": "Operação tributável",
	"2": "Imunidade",
	"3": "Exportação de serviço",
	"4": "Não Incidência",
}

// retencaoISSQN describes TSTipoRetISSQN.
var retencaoISSQN = map[string]string{
	"1": "Não Retido",
	"2": "Retido pelo Tomador",
	"3": "Retido pelo Intermediário",
}

// regimeEspecial describes TSRegEspTrib (prest/regTrib/regEspTrib).
var regimeEspecial = map[string]string{
	"0": "Nenhum",
	"1": "Ato Cooperado (Cooperativa)",
	"2": "Estimativa",
	"3": "Microempresa Municipal",
	"4": "Notário ou Registrador",
	"5": "Profissional Autônomo",
	"6": "Sociedade de Profissionais",
	"9": "Outros",
}

// imunidadeISSQN describes TSTipoImunidadeISSQN. The constitutional wording is
// kept as the schema documents it; the field truncates at 40 characters on the
// document, which the nota técnica shows in its own example.
var imunidadeISSQN = map[string]string{
	"0": "Imunidade (tipo não informado na nota de origem)",
	"1": "Patrimônio, renda ou serviços, uns dos outros (CF88, Art 150, VI, a)",
	"2": "Templos de qualquer culto (CF88, Art 150, VI, b)",
	"3": "Patrimônio, renda ou serviços dos partidos políticos, inclusive suas fundações, " +
		"das entidades sindicais dos trabalhadores, das instituições de educação e de " +
		"assistência social, sem fins lucrativos (CF88, Art 150, VI, c)",
	"4": "Livros, jornais, periódicos e o papel destinado a sua impressão (CF88, Art 150, VI, d)",
	"5": "Fonogramas e videofonogramas musicais produzidos no Brasil (CF88, Art 150, VI, e)",
}

// exigibilidadeSuspensa describes TSOpExigSuspensa. NT 008 dictates the exact
// sentences for this field.
var exigibilidadeSuspensa = map[string]string{
	"1": "Exigibilidade Suspensa por Decisão Judicial",
	"2": "Exigibilidade Suspensa por Processo Administrativo",
}

// beneficioMunicipal describes TBMISSQN (valores/tpBM).
var beneficioMunicipal = map[string]string{
	"1": "Isenção",
	"2": "Redução da Base de Cálculo em %",
	"3": "Redução da Base de Cálculo em R$",
	"4": "Alíquota Diferenciada",
}

// retencaoPisCofins describes TSTipoRetPISCofins.
var retencaoPisCofins = map[string]string{
	"0": "PIS/COFINS/CSLL Não Retidos",
	"1": "PIS/COFINS Retidos",
	"2": "PIS/COFINS Não Retidos",
	"3": "PIS/COFINS/CSLL Retidos",
	"4": "PIS/COFINS Retidos, CSLL Não Retido",
	"5": "PIS Retido, COFINS/CSLL Não Retido",
	"6": "COFINS Retido, PIS/CSLL Não Retido",
	"7": "PIS Não Retido, COFINS/CSLL Retidos",
	"8": "PIS/COFINS Não Retidos, CSLL Retido",
	"9": "COFINS Não Retido, PIS/CSLL Retidos",
}

// simplesNacional describes TSOpSimpNac (prest/regTrib/opSimpNac).
var simplesNacional = map[string]string{
	"1": "Não Optante",
	"2": "Optante - Microempreendedor Individual (MEI)",
	"3": "Optante - Microempresa ou Empresa de Pequeno Porte (ME/EPP)",
}

// apuracaoSimplesNacional describes TSRegimeApuracaoSimpNac.
var apuracaoSimplesNacional = map[string]string{
	"1": "Regime de apuração dos tributos federais e municipal pelo Simples Nacional",
	"2": "Regime de apuração dos tributos federais pelo Simples Nacional e ISSQN por fora, " +
		"conforme a legislação municipal",
	"3": "Regime de apuração dos tributos federais e municipal por fora do Simples Nacional, " +
		"conforme as legislações de cada tributo",
}

// paisBrasil is Brazil in the two-letter ISO table NT 008 prints countries in.
const paisBrasil = "BR"

// ufPorCodigo maps the first two digits of an IBGE municipality code — the
// state's own IBGE code — to its abbreviation.
var ufPorCodigo = map[string]string{
	"11": "RO", "12": "AC", "13": "AM", "14": "RR", "15": "PA", "16": "AP", "17": "TO",
	"21": "MA", "22": "PI", "23": "CE", "24": "RN", "25": "PB", "26": "PE", "27": "AL",
	"28": "SE", "29": "BA",
	"31": "MG", "32": "ES", "33": "RJ", "35": "SP",
	"41": "PR", "42": "SC", "43": "RS",
	"50": "MS", "51": "MT", "52": "GO", "53": "DF",
}

// siglaUF returns the state of a seven-digit IBGE code, or "" for anything
// else.
func siglaUF(codigo string) string {
	if len(codigo) != 7 || !apenasDigitos(codigo) {
		return ""
	}
	return ufPorCodigo[codigo[:2]]
}
