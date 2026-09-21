package validation

import (
	"strings"
	"testing"
)

// comSubst inserts a subst element into the valid document, which is the only
// way to reach this validation: the CLI refuses a bad reason before building
// anything, so a hand-written XML is what exercises the schema check.
func comSubst(subst string) string {
	return strings.Replace(validDPSXML,
		"    <prest>", subst+"\n    <prest>", 1)
}

const substValida = `    <subst>
      <chSubstda>41069022212345678000195000000000000126081234567890</chSubstda>
      <cMotivo>01</cMotivo>
    </subst>`

func TestValidateSubst_Aceita(t *testing.T) {
	erros := NewStructuralValidator().ValidateDPS(comSubst(substValida))
	for _, e := range erros {
		if strings.Contains(e.Element, "subst") {
			t.Errorf("subst valido recusado: %s — %s", e.Element, e.Message)
		}
	}
}

// Um código de cancelamento ("1") é uma string não vazia e passaria numa
// checagem de presença. Só a enumeração do TSCodJustSubst o recusa.
func TestValidateSubst_RecusaCodigoDeCancelamento(t *testing.T) {
	xml := comSubst(`    <subst>
      <chSubstda>41069022212345678000195000000000000126081234567890</chSubstda>
      <cMotivo>1</cMotivo>
    </subst>`)

	erros := NewStructuralValidator().ValidateDPS(xml)
	if !temErroEm(erros, "infDPS/subst/cMotivo") {
		t.Errorf("cMotivo=1 deveria ser recusado; erros: %v", erros)
	}
}

func TestValidateSubst_RecusaChaveMalFormada(t *testing.T) {
	for _, chave := range []string{"123", "NFSe4106902221234567800019500000000000126081234567890", strings.Repeat("9", 51)} {
		xml := comSubst(`    <subst>
      <chSubstda>` + chave + `</chSubstda>
      <cMotivo>99</cMotivo>
    </subst>`)

		erros := NewStructuralValidator().ValidateDPS(xml)
		if !temErroEm(erros, "infDPS/subst/chSubstda") {
			t.Errorf("a chave %q deveria ser recusada; erros: %v", chave, erros)
		}
	}
}

// subst é opcional: uma emissão comum não carrega o elemento e não pode ser
// penalizada por isso.
func TestValidateSubst_AusenteEValido(t *testing.T) {
	erros := NewStructuralValidator().ValidateDPS(validDPSXML)
	for _, e := range erros {
		if strings.Contains(e.Element, "subst") {
			t.Errorf("a ausencia de subst gerou erro: %s — %s", e.Element, e.Message)
		}
	}
}

func temErroEm(errs []StructuralError, elemento string) bool {
	for _, e := range errs {
		if e.Element == elemento {
			return true
		}
	}
	return false
}
