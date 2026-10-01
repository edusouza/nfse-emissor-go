// Package rejeicao says what a rejection code of the Sefin Nacional means, in
// the government's own words: the rule behind it, the field of the DPS it is
// about, and whether it depends on the municipality.
//
// The Sefin answers a rejection with a code and one sentence. The sentence
// rarely says which field is wrong, and never says the condition that failed;
// ANEXO I does. The table is generated from it (gerar.go) and checked against
// it by the tests: nobody writes a code here by hand (ADR 0019).
package rejeicao

import "strings"

//go:generate go run gerar.go

// Regra is one rule of ANEXO I that answers with a given code.
type Regra struct {
	// Mensagem is the sentence the Sefin answers with.
	Mensagem string

	// Campo is the path the annex gives, from the NFS-e (NFSe/infNFSe/...).
	// Empty for the rules applied to the transmission itself.
	Campo string

	// Texto is the rule, as the annex states it.
	Texto string

	// Nivel is "1" (layout), "2" (general rule) or "3" (depends on the
	// municipality); empty for the rules applied to the transmission.
	Nivel string
}

// prefixoNFSe is where the annex roots the paths. The DPS the user holds is
// the part of the NFS-e below it.
const prefixoNFSe = "NFSe/infNFSe/"

// CampoNaDPS is the path as it reads in the user's own file, starting at DPS.
// A field the Sefin fills in the NFS-e, outside the DPS, keeps the full path.
func (r Regra) CampoNaDPS() string {
	if resto, ok := strings.CutPrefix(r.Campo, prefixoNFSe); ok && strings.HasPrefix(resto, "DPS/") {
		return resto
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

// Buscar returns the rules behind a code. mensagem is what the Sefin answered:
// for the few codes the annex gives to more than one rule, it picks the rule
// that was broken. When it matches none, every rule is returned, because
// guessing would hide the right one.
func Buscar(codigo, mensagem string) []Regra {
	regras := anexoI[strings.ToUpper(strings.TrimSpace(codigo))]
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
