// Package rejeicao says what a rejection code of the Sefin Nacional means, in
// the government's own words: the rule behind it, the field it is about, and
// whether it depends on the municipality.
//
// The Sefin answers a rejection with a code and one sentence. The sentence
// rarely says which field is wrong, and never says the condition that failed;
// the annexes do. ANEXO I covers the DPS, ANEXO II the event requests, such as
// a cancellation. The tables are generated from them (gerar.go) and checked
// against them by the tests: nobody writes a code here by hand (ADR 0019).
//
// The two tables are kept apart. A few codes appear in both, for the same
// rule applied to a different document, and only the annex the answer came
// from says which field it is about.
package rejeicao

import "strings"

//go:generate go run gerar.go

// Regra is one rule of an annex that answers with a given code.
type Regra struct {
	// Mensagem is the sentence the Sefin answers with.
	Mensagem string

	// Campo is the path the annex gives, from the document the Sefin
	// produces: NFSe/infNFSe/... in ANEXO I, evento/... in ANEXO II. Empty
	// for the rules applied to the transmission itself.
	Campo string

	// Texto is the rule, as the annex states it.
	Texto string

	// Nivel is "1" (layout), "2" (general rule) or "3" (depends on the
	// municipality); empty when the annex gives none.
	Nivel string
}

// raizes are where the annexes root their paths, and the part the user
// holds below each: the DPS inside the NFS-e, the request inside the event.
var raizes = []struct{ documento, arquivo string }{
	{"NFSe/infNFSe/", "DPS/"},
	{"evento/", "pedRegEvento/"},
}

// CampoNoArquivo is the path as it reads in what the user sent: from DPS for
// an emission, from pedRegEvento for an event request. A field the Sefin
// fills in its own document, outside what was sent, keeps the full path.
func (r Regra) CampoNoArquivo() string {
	for _, raiz := range raizes {
		if resto, ok := strings.CutPrefix(r.Campo, raiz.documento); ok && strings.HasPrefix(resto, raiz.arquivo) {
			return resto
		}
	}
	return r.Campo
}

// DependeDoMunicipio reports a level 3 rule: one whose outcome follows the
// parameters the municipality set in the national system, so that the same
// DPS can pass in one municipality and fail in another.
func (r Regra) DependeDoMunicipio() bool { return r.Nivel == "3" }

// AcrescentaAMensagem reports whether the rule says more than the sentence
// the Sefin answered with. Often it is the same sentence, reworded slightly;
// repeating it under the message would only add noise.
func (r Regra) AcrescentaAMensagem() bool {
	return r.Texto != "" && normalizar(r.Texto) != normalizar(r.Mensagem)
}

// BuscarDPS returns the rules of ANEXO I behind a code the Sefin answered to
// a DPS. mensagem is what the Sefin said: for the few codes the annex gives to
// more than one rule, it picks the rule that was broken. When it matches
// none, every rule is returned, because guessing would hide the right one.
func BuscarDPS(codigo, mensagem string) []Regra { return buscar(anexoI, codigo, mensagem) }

// BuscarEvento is BuscarDPS for a code the Sefin answered to an event
// request, from ANEXO II.
func BuscarEvento(codigo, mensagem string) []Regra { return buscar(anexoII, codigo, mensagem) }

func buscar(tabela map[string][]Regra, codigo, mensagem string) []Regra {
	regras := tabela[strings.ToUpper(strings.TrimSpace(codigo))]
	if len(regras) < 2 || mensagem == "" {
		return regras
	}
	var escolhidas []Regra
	for _, r := range regras {
		if normalizar(r.Mensagem) == normalizar(mensagem) {
			escolhidas = append(escolhidas, r)
		}
	}
	if len(escolhidas) == 0 {
		return regras
	}
	return escolhidas
}

// normalizar compares sentences the way a reader would: the annex wraps lines
// inside its cells, and ends some sentences with a period and others not.
func normalizar(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.ToLower(strings.TrimRight(s, ". "))
}
