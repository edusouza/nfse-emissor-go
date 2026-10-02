package servico

import (
	"fmt"
	"strings"
)

// ComplementoPadrao is the municipal complement of a service the municipality
// did not break down any further, which is most of them.
const ComplementoPadrao = "000"

// CodigoCompleto builds the code the ADN municipal parameters API keys every
// service on: cTribNac followed by the municipality's three-digit complement,
// dotted as "II.SS.DD.CCC".
//
// The service accepts no other spelling. "010701" and "010701000" were both
// refused with 400 when checked against it (docs/convenio-municipal.md), so the
// dots are part of the contract, not decoration.
//
// codigo is a cTribNac in any of the ways a person writes one — "010701",
// "01.07.01", or "10701" as a spreadsheet shows it — or a full code already
// carrying its complement. An empty complemento means ComplementoPadrao; one
// that contradicts the complement inside codigo is refused rather than picked
// over it.
//
// The cTribNac must be in the national list this binary carries: asking the
// government about a code that does not exist costs a round trip to learn the
// same thing.
func CodigoCompleto(codigo, complemento string) (string, error) {
	digitos := somenteDigitos(codigo)
	if digitos == "" {
		return "", fmt.Errorf("codigo de servico %q invalido: informe o cTribNac de 6 digitos, como 010701 ou 01.07.01", codigo)
	}

	// A leading zero is what a spreadsheet drops, so one missing digit is
	// restored the way PorCodigo does it.
	switch len(digitos) {
	case 5, 8:
		digitos = "0" + digitos
	}

	var cTribNac, embutido string
	switch len(digitos) {
	case 6:
		cTribNac = digitos
	case 9:
		cTribNac, embutido = digitos[:6], digitos[6:]
	default:
		return "", fmt.Errorf("codigo de servico %q invalido: sao 6 digitos do cTribNac, "+
			"ou 9 com o complemento municipal (01.07.01.000)", codigo)
	}

	complemento = strings.TrimSpace(complemento)
	if complemento != "" && !complementoValido(complemento) {
		return "", fmt.Errorf("complemento municipal %q invalido: sao 3 digitos, como 000 ou 001", complemento)
	}
	switch {
	case embutido != "" && complemento != "" && complemento != embutido:
		return "", fmt.Errorf("o codigo %q ja traz o complemento %s, diferente do informado (%s); informe um so",
			codigo, embutido, complemento)
	case embutido != "":
		complemento = embutido
	case complemento == "":
		complemento = ComplementoPadrao
	}

	if _, ok := porCode[cTribNac]; !ok {
		return "", fmt.Errorf("o cTribNac %s nao esta na lista nacional de servicos (%s); "+
			"procure o codigo certo com 'nfse servico'", cTribNac, Anexo)
	}

	return fmt.Sprintf("%s.%s.%s.%s", cTribNac[0:2], cTribNac[2:4], cTribNac[4:6], complemento), nil
}

// somenteDigitos drops separators, returning "" when anything other than
// digits, dots, spaces or hyphens is present: a letter in a code is a typo to
// report, not noise to strip.
func somenteDigitos(valor string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(valor) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == ' ' || r == '-':
		default:
			return ""
		}
	}
	return b.String()
}

func complementoValido(valor string) bool {
	if len(valor) != 3 {
		return false
	}
	for _, r := range valor {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
