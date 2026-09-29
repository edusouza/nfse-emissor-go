package esquema

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// primitivos are the XSD built-in types the schemas use, with the lexical
// check each one applies before any facet. A built-in missing from here makes
// the schema fail to load.
var primitivos = map[string]func(string) bool{
	"string":       func(string) bool { return true },
	"anyURI":       func(string) bool { return true },
	"ID":           regexp.MustCompile(`^[\pL_][\pL\pN._\-]*$`).MatchString,
	"integer":      regexp.MustCompile(`^[+-]?[0-9]+$`).MatchString,
	"decimal":      regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)$`).MatchString,
	"date":         dataValida,
	"base64Binary": base64Valido,
}

var formatoData = regexp.MustCompile(`^([0-9]{4})-([0-9]{2})-([0-9]{2})(Z|[+-]([0-9]{2}):([0-9]{2}))?$`)

// dataValida accepts xs:date: a calendar date that exists, in a year other
// than 0000, with an optional time zone within ±14:00. The emitter only
// writes four-digit years.
func dataValida(v string) bool {
	m := formatoData.FindStringSubmatch(v)
	if m == nil || m[1] == "0000" {
		return false
	}
	if _, err := time.Parse("2006-01-02", m[1]+"-"+m[2]+"-"+m[3]); err != nil {
		return false
	}
	if m[5] != "" {
		horas, minutos := atoi2(m[5]), atoi2(m[6])
		if minutos > 59 || horas > 14 || (horas == 14 && minutos != 0) {
			return false
		}
	}
	return true
}

func atoi2(s string) int {
	return int(s[0]-'0')*10 + int(s[1]-'0')
}

func base64Valido(v string) bool {
	limpo := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, v)
	// Strict: XSD's canonical lexical space leaves no stray bits in the
	// padding, so "AB==" is not base64Binary.
	_, err := base64.StdEncoding.Strict().DecodeString(limpo)
	return err == nil
}

// traduzirPadrao turns an XSD pattern into a Go regular expression.
//
// An XSD pattern always matches the whole value, so it is anchored here. The
// two dialects agree on most syntax and differ on a few points, handled as
// the pattern is copied:
//
//   - \d is any Unicode decimal digit in XSD and ASCII in Go: it becomes
//     \p{Nd}, and \D becomes \P{Nd}.
//   - . excludes \r as well as \n in XSD: it becomes [^\n\r].
//   - ^ and $ are ordinary characters in XSD: they are escaped — except when
//     they wrap the whole pattern. TSSerieDPS is "^0{0,4}\d{1,5}$", which
//     read literally would make every series invalid; the Sefin reads the
//     two as anchors (a DPS with series 00001 was authorized on 18/09/2026),
//     and so does this.
//
// Syntax that exists in only one of the dialects, or means something else in
// each (\w, \b, \i, \c, class subtraction, (?...), lazy quantifiers), is
// refused, so that a schema using it fails to load instead of being checked
// by a rule nobody wrote.
func traduzirPadrao(p string) (*regexp.Regexp, error) {
	corpo := p
	if len(corpo) >= 2 && corpo[0] == '^' && corpo[len(corpo)-1] == '$' && !strings.HasSuffix(corpo, `\$`) {
		corpo = corpo[1 : len(corpo)-1]
	}

	var b strings.Builder
	runas := []rune(corpo)
	naClasse := false
	for i := 0; i < len(runas); i++ {
		r := runas[i]
		switch {
		case r == '\\':
			if i+1 >= len(runas) {
				return nil, fmt.Errorf("%q termina com uma barra invertida", p)
			}
			i++
			switch e := runas[i]; e {
			case 'd':
				b.WriteString(`\p{Nd}`)
			case 'D':
				if naClasse {
					return nil, fmt.Errorf("%q usa \\D dentro de uma classe", p)
				}
				b.WriteString(`\P{Nd}`)
			case 's', 'S', 'n', 'r', 't', 'p', 'P',
				'\\', '|', '.', '-', '^', '?', '*', '+', '{', '}', '(', ')', '[', ']', '$':
				b.WriteRune('\\')
				b.WriteRune(e)
			default:
				return nil, fmt.Errorf("%q usa \\%c, que XSD e Go leem de jeitos diferentes", p, e)
			}
		case naClasse:
			if r == '[' {
				return nil, fmt.Errorf("%q usa subtracao ou aninhamento de classes, que so existe em XSD", p)
			}
			if r == ']' {
				naClasse = false
			}
			b.WriteRune(r)
		case r == '[':
			naClasse = true
			b.WriteRune(r)
			// A ] right after [ or [^ is a literal in both dialects.
			if i+1 < len(runas) && runas[i+1] == '^' {
				i++
				b.WriteRune('^')
			}
			if i+1 < len(runas) && runas[i+1] == ']' {
				i++
				b.WriteString(`\]`)
			}
		case r == '.':
			b.WriteString(`[^\n\r]`)
		case r == '^' || r == '$':
			b.WriteRune('\\')
			b.WriteRune(r)
		case r == '(' && i+1 < len(runas) && runas[i+1] == '?':
			return nil, fmt.Errorf("%q usa (?, que so existe em Go", p)
		case r == '?' && i > 0 && strings.ContainsRune("*+?}", runas[i-1]) && !escapado(runas, i-1):
			return nil, fmt.Errorf("%q usa um quantificador preguicoso, que so existe em Go", p)
		default:
			b.WriteRune(r)
		}
	}
	if naClasse {
		return nil, fmt.Errorf("%q abre uma classe e nao fecha", p)
	}
	return regexp.Compile(`^(?:` + b.String() + `)$`)
}

// escapado reports whether the rune at i is preceded by an odd number of
// backslashes.
func escapado(runas []rune, i int) bool {
	n := 0
	for j := i - 1; j >= 0 && runas[j] == '\\'; j-- {
		n++
	}
	return n%2 == 1
}

// espacos returns the whiteSpace in force: the nearest level that sets it.
func (t *tipoSimples) espacosEfetivos() string {
	for n := t; n != nil; n = n.base {
		if n.espacos != "" {
			return n.espacos
		}
	}
	return "preserve"
}

func normalizarEspacos(v, modo string) string {
	if modo == "preserve" {
		return v
	}
	v = strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, v)
	if modo == "collapse" {
		v = strings.Join(strings.Fields(v), " ")
	}
	return v
}

// validar returns what is wrong with the value, or "" when it is valid.
// Every level of the derivation is checked: each one only narrows.
func (t *tipoSimples) validar(valor string) string {
	v := normalizarEspacos(valor, t.espacosEfetivos())
	tamanho := utf8.RuneCountInString(v)

	for n := t; n != nil; n = n.base {
		if n.enumeracoes != nil && !contem(n.enumeracoes, v) {
			return fmt.Sprintf("%q nao e um dos valores de %s: %s", v, n.nome.local, listar(n.enumeracoes))
		}
		if len(n.padroes) > 0 && !algumCasa(n.padroes, v) {
			return fmt.Sprintf("%q nao segue o formato de %s (%s)%s", v, n.nome.local, fontes(n.padroes), dica(v))
		}
		if n.exato >= 0 && tamanho != n.exato {
			return fmt.Sprintf("tem %d caracteres; %s exige exatamente %d", tamanho, n.nome.local, n.exato)
		}
		if n.minimo >= 0 && tamanho < n.minimo {
			return fmt.Sprintf("tem %d caracteres; %s exige no minimo %d", tamanho, n.nome.local, n.minimo)
		}
		if n.maximo >= 0 && tamanho > n.maximo {
			return fmt.Sprintf("tem %d caracteres; %s aceita no maximo %d", tamanho, n.nome.local, n.maximo)
		}
		if n.base == nil && !primitivos[n.builtin](v) {
			return fmt.Sprintf("%q nao e um xs:%s valido", v, n.builtin)
		}
	}
	return ""
}

// dica explains the two ways a free-text field usually breaks TSString, whose
// pattern is unreadable: only Latin-1 characters, and no space at either end.
func dica(v string) string {
	for _, r := range v {
		if r > 0xFF {
			return fmt.Sprintf("; o caractere %q esta fora do Latin-1 que o schema aceita", r)
		}
	}
	if v != strings.TrimSpace(v) {
		return "; o valor comeca ou termina com espaco"
	}
	return ""
}

func contem(lista []string, v string) bool {
	for _, item := range lista {
		if item == v {
			return true
		}
	}
	return false
}

func algumCasa(padroes []padrao, v string) bool {
	for _, p := range padroes {
		if p.re.MatchString(v) {
			return true
		}
	}
	return false
}

func fontes(padroes []padrao) string {
	var s []string
	for _, p := range padroes {
		s = append(s, p.fonte)
	}
	return strings.Join(s, " | ")
}

// listar shows the allowed values, cut short when the list is long: some
// enumerations in the schema run to hundreds of entries.
func listar(valores []string) string {
	const limite = 12
	if len(valores) <= limite {
		return strings.Join(valores, ", ")
	}
	return strings.Join(valores[:limite], ", ") + fmt.Sprintf(" e mais %d", len(valores)-limite)
}
