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

var formatoData = regexp.MustCompile(`^([0-9]{4})-([0-9]{2})-([0-9]{2})(Z|[+-][0-9]{2}:[0-9]{2})?$`)

// dataValida accepts xs:date: a calendar date that exists, with an optional
// time zone. The emitter only writes four-digit years.
func dataValida(v string) bool {
	m := formatoData.FindStringSubmatch(v)
	if m == nil {
		return false
	}
	_, err := time.Parse("2006-01-02", m[1]+"-"+m[2]+"-"+m[3])
	return err == nil
}

func base64Valido(v string) bool {
	limpo := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, v)
	_, err := base64.StdEncoding.DecodeString(limpo)
	return err == nil
}

// traduzirPadrao turns an XSD pattern into a Go regular expression.
//
// An XSD pattern always matches the whole value, so it is anchored here. The
// schemas are otherwise written in the common subset of both dialects, with
// one exception: TSSerieDPS is "^0{0,4}\d{1,5}$". In XSD, ^ and $ are
// ordinary characters, and read that way no series could ever be valid. The
// Sefin reads them as anchors — a DPS with series 00001 was authorized on
// 18/09/2026 — so a pattern wrapped in both is read the same way.
func traduzirPadrao(p string) (*regexp.Regexp, error) {
	corpo := p
	if len(corpo) >= 2 && strings.HasPrefix(corpo, "^") && strings.HasSuffix(corpo, "$") && !strings.HasSuffix(corpo, `\$`) {
		corpo = corpo[1 : len(corpo)-1]
	}
	for _, xsdOnly := range []string{`\i`, `\I`, `\c`, `\C`, `-[`} {
		if strings.Contains(corpo, xsdOnly) {
			return nil, fmt.Errorf("%q usa %s, que so existe em XSD", p, xsdOnly)
		}
	}
	return regexp.Compile(`^(?:` + corpo + `)$`)
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
