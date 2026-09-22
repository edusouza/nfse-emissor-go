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
