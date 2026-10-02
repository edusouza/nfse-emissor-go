// Package parametrizacao queries the ADN municipal parameters API: whether a
// municipality's convênio with the national system is active, and the ISSQN
// rate it set for a service.
//
// Everything here is traceable to docs/api/adn-parametrizacao-swagger.json
// ("API NFS-e - ADN Parâmetros Municipais") and to the checks against the live
// service recorded in docs/convenio-municipal.md, where the swagger is silent
// on formats (ADR 0005). Where the two disagree, the live service wins, and the
// comment says so.
//
// Only the GET routes are implemented. The POST ones let a municipality
// maintain its own parameters; a taxpayer has no business calling them.
package parametrizacao

// Environment names, spelled the way nfse.yaml spells them.
const (
	AmbienteProducao         = "producao"
	AmbienteProducaoRestrita = "producao-restrita"
)

// Base URLs. The swagger declares only the relative server "/parametrizacao";
// the hosts are the ADN's, and the production document is byte for byte the
// same as the restricted one.
const (
	// ProducaoBaseURL answers with the parameters that are in force.
	ProducaoBaseURL = "https://adn.nfse.gov.br/parametrizacao"

	// ProducaoRestritaBaseURL is the government's testing environment. Its
	// data is test data: Curitiba's rate history there has 5% → 2% for one day
	// → 5%. Trust its format, not its values.
	ProducaoRestritaBaseURL = "https://adn.producaorestrita.nfse.gov.br/parametrizacao"
)

// LayoutCompetencia is how a competence date travels in the path.
//
// The swagger says only "date-time". The service accepts other spellings, but
// reads an ambiguous one month first: 12-31-2022 is 31 December, and
// 15-09-2026 is a 400. A day-month-year date with the day up to 12 would be
// accepted and read wrong without a word, so only ISO dates ever leave here.
const LayoutCompetencia = "2006-01-02"

// Route suffixes, appended after the path parameters:
//
//	GET /{codigoMunicipio}/convenio
//	GET /{codigoMunicipio}/{codigoServico}/{competencia}/aliquota
//	GET /{codigoMunicipio}/{codigoServico}/historicoaliquotas
const (
	rotaConvenio           = "/convenio"
	rotaAliquota           = "/aliquota"
	rotaHistoricoAliquotas = "/historicoaliquotas"
)
