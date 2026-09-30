// Package texto carries the text folding the emitter's lookups share.
//
// The national service list and the municipality names are both searched by
// what someone types in a terminal, which is rarely what the government
// wrote: no accents, any case. Folding lives here rather than in each package
// so that there is one table to fix when a character is missing from it — two
// copies would mean fixing one and shipping the other.
package texto

import "strings"

// Dobrar lowercases text and strips the accents Portuguese uses, so that a
// query typed without them still matches.
//
// A replacer is enough for Portuguese and keeps the dependency list where it
// is; golang.org/x/text would do this properly for every script, at the cost
// of a dependency in a binary that handles a private key.
func Dobrar(s string) string {
	return acentos.Replace(strings.ToLower(s))
}

// Chave reduces a name to the letters and digits in it, folded, so that the
// ways people write one name compare equal: "Alta Floresta D'Oeste", "alta
// floresta d oeste" and "ALTA FLORESTA DOESTE" are one key. Dropping the
// spaces as well as the punctuation is what makes the apostrophe forms agree.
func Chave(s string) string {
	var b strings.Builder
	for _, r := range Dobrar(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var acentos = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)
