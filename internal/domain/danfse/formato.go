package danfse

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Traco is what a field with nothing behind it prints — note 12 of NT 008:
// "Os campos sem informações no XML devem ser preenchidos com um traço (-)".
const Traco = "-"

// campo applies note 12 to a value.
func campo(valor string) string {
	if strings.TrimSpace(valor) == "" {
		return Traco
	}
	return valor
}

// data formats TSData (AAAA-MM-DD) as the DD/MM/AAAA the document asks for.
//
// A value that does not parse is passed through untouched rather than dropped:
// the invoice is the government's, it was accepted by the schema, and hiding
// what it says would be worse than printing it in an unexpected shape.
func data(valor string) string {
	valor = strings.TrimSpace(valor)
	if valor == "" {
		return ""
	}
	quando, err := time.Parse("2006-01-02", valor)
	if err != nil {
		return valor
	}
	return quando.Format("02/01/2006")
}

// dataHora formats TSDateTimeUTC (AAAA-MM-DDThh:mm:ssTZD) as
// DD/MM/AAAA hh:mm:ss.
//
// The offset the invoice carries is kept — time.Parse builds a fixed zone out
// of it, so the printed clock is the one the document states, not the one of
// whoever happens to be printing it.
func dataHora(valor string) string {
	valor = strings.TrimSpace(valor)
	if valor == "" {
		return ""
	}
	quando, err := time.Parse(time.RFC3339, valor)
	if err != nil {
		return valor
	}
	return quando.Format("02/01/2006 15:04:05")
}

// limitar truncates a value to at most max characters, ending it with an
// ellipsis, as NT 008 asks field by field ("Preencher com reticências (...),
// caso a descrição supere N caracteres").
//
// The count is in characters, not bytes: "São Paulo" is nine characters and
// eleven bytes, and a byte-based cut would both shorten the text and be able to
// split an accented letter in half.
func limitar(valor string, max int) string {
	if max <= 0 || utf8.RuneCountInString(valor) <= max {
		return valor
	}
	if max <= 3 {
		return string([]rune(valor)[:max])
	}
	return string([]rune(valor)[:max-3]) + "..."
}

// moeda formats a TSDec15V2 value the way a Brazilian document reads it:
// 1500.00 in the XML becomes 1.500,00 on the paper.
//
// The conversion is textual, never through a float: money in a fiscal document
// has to come out exactly as the invoice states it, and binary floating point
// is the classic way to turn 1234.10 into 1234.0999999.
func moeda(valor string) string {
	inteiro, decimal, ok := partesDoNumero(valor)
	if !ok {
		return valor
	}
	return agrupar(inteiro) + "," + decimal
}

// percentual formats a rate. It keeps the two decimals the schema's TSDec2V2
// carries and adds the sign the reader expects.
func percentual(valor string) string {
	inteiro, decimal, ok := partesDoNumero(valor)
	if !ok {
		return valor
	}
	return agrupar(inteiro) + "," + decimal + "%"
}

// partesDoNumero splits a decimal into its integer and fractional halves,
// always with two digits after the comma, and reports whether the value looked
// like a number at all.
func partesDoNumero(valor string) (inteiro, decimal string, ok bool) {
	valor = strings.TrimSpace(valor)
	if valor == "" {
		return "", "", false
	}

	sinal := ""
	if strings.HasPrefix(valor, "-") {
		sinal, valor = "-", valor[1:]
	}

	inteiro, decimal = valor, "00"
	if ponto := strings.IndexByte(valor, '.'); ponto >= 0 {
		inteiro, decimal = valor[:ponto], valor[ponto+1:]
	}
	if inteiro == "" {
		inteiro = "0"
	}

	if !apenasDigitos(inteiro) || !apenasDigitos(decimal) {
		return "", "", false
	}

	switch {
	case len(decimal) < 2:
		decimal += strings.Repeat("0", 2-len(decimal))
	case len(decimal) > 2:
		// The schema allows more than two decimal places in some rates; the
		// document shows two, and cutting is safer than rounding a value the
		// government already decided.
		decimal = decimal[:2]
	}

	return sinal + inteiro, decimal, true
}

// agrupar puts a dot every three digits, from the right.
func agrupar(inteiro string) string {
	sinal := ""
	if strings.HasPrefix(inteiro, "-") {
		sinal, inteiro = "-", inteiro[1:]
	}

	var partes []string
	for len(inteiro) > 3 {
		partes = append([]string{inteiro[len(inteiro)-3:]}, partes...)
		inteiro = inteiro[:len(inteiro)-3]
	}
	partes = append([]string{inteiro}, partes...)

	return sinal + strings.Join(partes, ".")
}

func apenasDigitos(valor string) bool {
	if valor == "" {
		return false
	}
	for _, r := range valor {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
