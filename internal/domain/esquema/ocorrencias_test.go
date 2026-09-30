package esquema

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

// esquemaSintetico loads a one-element schema whose content model is given.
func esquemaSintetico(t *testing.T, modelo string) *Esquema {
	t.Helper()

	xsd := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:t" xmlns="urn:t" elementFormDefault="qualified">
  <xs:simpleType name="Vazio"><xs:restriction base="xs:string"><xs:maxLength value="0"/></xs:restriction></xs:simpleType>
  <xs:element name="r"><xs:complexType>` + modelo + `</xs:complexType></xs:element>
</xs:schema>`
	e, err := Carregar(fstest.MapFS{"s.xsd": {Data: []byte(xsd)}}, "s.xsd")
	if err != nil {
		t.Fatalf("schema sintetico nao carregou: %v", err)
	}
	return e
}

func documento(n int) []byte {
	return []byte(`<r xmlns="urn:t">` + strings.Repeat(`<a/>`, n) + `</r>`)
}

// The matcher is held against a regular expression over the same content
// model, for every bound and length in range. Occurrence bounds above one
// do not appear in the NFS-e schemas today, which is why this is checked
// by brute force rather than trusted: a future schema would otherwise be
// misvalidated in silence.
func TestOcorrenciasContraExpressaoRegular(t *testing.T) {
	const a = `<xs:element name="a" type="Vazio"/>`
	modelos := []struct {
		nome, escolha, regex string
	}{
		{"a ou aaa", `<xs:sequence>` + a + `</xs:sequence><xs:sequence>` + a + a + a + `</xs:sequence>`, `(a|aaa)`},
		{"aa ou aaa", `<xs:sequence>` + a + a + `</xs:sequence><xs:sequence>` + a + a + a + `</xs:sequence>`, `(aa|aaa)`},
		{"a opcional", `<xs:sequence><xs:element name="a" type="Vazio" minOccurs="0"/></xs:sequence>`, `(a?)`},
	}

	for _, m := range modelos {
		for min := 0; min <= 5; min++ {
			for _, max := range []string{"unbounded", fmt.Sprint(max(min, 1)), fmt.Sprint(min + 2)} {
				modelo := fmt.Sprintf(`<xs:choice minOccurs="%d" maxOccurs="%s">%s</xs:choice>`, min, max, m.escolha)
				e := esquemaSintetico(t, modelo)

				quant := fmt.Sprintf("{%d,%s}", min, strings.TrimPrefix(max, "unbounded"))
				if max == "unbounded" {
					quant = fmt.Sprintf("{%d,}", min)
				}
				ref := regexp.MustCompile(`^(?:` + m.regex + `)` + quant + `$`)

				for n := 0; n <= 16; n++ {
					quer := ref.MatchString(strings.Repeat("a", n))
					obtido := len(e.Validar(documento(n))) == 0
					if obtido != quer {
						t.Errorf("%s, min=%d max=%s, %d filhos: validador=%v, esperado=%v", m.nome, min, max, n, obtido, quer)
					}
				}
			}
		}
	}
}
