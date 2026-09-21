// Package texto carries the text folding the emitter's lookups share.
//
// Both the national service list and the municipality table are searched by
// what someone types in a terminal, which is rarely what the government wrote:
// no accents, any case. Folding lives here rather than in each package so that
// there is one table to fix when a character is missing from it — two copies
// would mean fixing one and shipping the other.
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

var acentos = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)
