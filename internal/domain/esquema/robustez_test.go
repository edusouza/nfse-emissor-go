package esquema

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestTraduzirPadrao(t *testing.T) {
	casos := []struct {
		padrao string
		aceita []string
		recusa []string
	}{
		// Anchored whole: an XSD pattern matches the entire value.
		{`[0-9]{3}`, []string{"123"}, []string{"1234", "a123"}},
		// ^...$ around the whole pattern are anchors (TSSerieDPS).
		{`^0{0,4}\d{1,5}$`, []string{"00001", "1"}, []string{"^00001$", "0000000001"}},
		// Anywhere else they are ordinary characters, as XSD reads them.
		{`a^b`, []string{"a^b"}, []string{"ab"}},
		{`a$`, []string{"a$"}, []string{"a"}},
		// \d is any decimal digit in XSD.
		{`\d{2}`, []string{"12", "١٢"}, []string{"ab"}},
		// . excludes \r in XSD.
		{`a.b`, []string{"a-b"}, []string{"a\rb", "a\nb"}},
		// Latin-1 range, as TSString writes it.
		{`[!-ÿ]{1}[ -ÿ]{0,}[!-ÿ]{1}|[!-ÿ]{1}`, []string{"Consultoria", "é"}, []string{" x", "x ", "—"}},
		// A ] right after [ is a literal.
		{`[]a]+`, []string{"]a"}, []string{"b"}},
	}
	for _, c := range casos {
		re, err := traduzirPadrao(c.padrao)
		if err != nil {
			t.Errorf("traduzirPadrao(%q): %v", c.padrao, err)
			continue
		}
		for _, v := range c.aceita {
			if !re.MatchString(v) {
				t.Errorf("%q deveria aceitar %q", c.padrao, v)
			}
		}
		for _, v := range c.recusa {
			if re.MatchString(v) {
				t.Errorf("%q deveria recusar %q", c.padrao, v)
			}
		}
	}

	// Syntax the two dialects read differently fails to load.
	for _, p := range []string{`\w+`, `\bx`, `\i\c*`, `[a-z-[aeiou]]`, `(?i)x`, `a*?`, `a+?`, `a{1,2}?`, `[ab`, `x\`} {
		if _, err := traduzirPadrao(p); err == nil {
			t.Errorf("traduzirPadrao(%q) deveria recusar", p)
		}
	}
	// An escaped quantifier followed by ? is not a lazy quantifier.
	if _, err := traduzirPadrao(`a\*?`); err != nil {
		t.Errorf(`a\*? foi recusado: %v`, err)
	}
}

func TestPrimitivos(t *testing.T) {
	data := map[string]bool{
		"2026-09-29": true, "2024-02-29": true, "2026-09-29Z": true, "2026-09-29-03:00": true, "2026-09-29+14:00": true,
		"2026-02-30": false, "0000-01-01": false, "2026-09-29+14:30": false, "2026-09-29-03:60": false,
		"2026-9-29": false, "26-09-29": false,
	}
	for v, quer := range data {
		if dataValida(v) != quer {
			t.Errorf("dataValida(%q) = %v", v, !quer)
		}
	}
	base64 := map[string]bool{"QUJD": true, "QUI=": true, "": true, "QU JD": true, "AB==": false, "QUJD=": false, "Q!JD": false}
	for v, quer := range base64 {
		if base64Valido(v) != quer {
			t.Errorf("base64Valido(%q) = %v", v, !quer)
		}
	}
}

// etree is lenient where XML is not. What it lets through has to be refused
// here, or a malformed document would pass as valid.
func TestDocumentoMalformado(t *testing.T) {
	base := dpsDoEmissor(t, nil)
	casos := map[string]string{
		"texto depois da raiz":   base + "lixo",
		"segundo elemento raiz":  base + "<DPS/>",
		"texto antes da raiz":    strings.Replace(base, "<DPS ", "lixo<DPS ", 1),
		"atributo repetido":      strings.Replace(base, `versao="1.00"`, `versao="9.99" versao="1.00"`, 1),
		"prefixo nao declarado":  strings.Replace(base, `versao="1.00"`, `foo:versao="1.00"`, 1),
		"xsi:type":               strings.Replace(base, `versao="1.00"`, `versao="1.00" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="x"`, 1),
		"espaco nao XML no meio": strings.Replace(base, "<tpAmb>", " <tpAmb>", 1),
	}
	for nome, doc := range casos {
		t.Run(nome, func(t *testing.T) {
			if erros := validarDPS(t, doc); len(erros) == 0 {
				t.Error("aceitou")
			}
		})
	}

	permitido := strings.Replace(base, `versao="1.00"`,
		`versao="1.00" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.sped.fazenda.gov.br/nfse DPS_v1.01.xsd"`, 1)
	for _, e := range validarDPS(t, permitido) {
		t.Errorf("xsi:schemaLocation deveria ser aceito: %v", e)
	}
}

// A document nested without end — the XML-DSig wildcard allows it — stops at
// a bounded depth instead of being walked in full.
func TestAninhamentoProfundo(t *testing.T) {
	e := esquemaSintetico(t, `<xs:sequence><xs:any namespace="##any" processContents="lax" minOccurs="0" maxOccurs="unbounded"/></xs:sequence>`)
	doc := `<r xmlns="urn:t">` + strings.Repeat(`<r>`, 500) + strings.Repeat(`</r>`, 500) + `</r>`
	erros := e.Validar([]byte(doc))
	if len(erros) == 0 || !strings.Contains(erros[0].Mensagem, "aninhamento") {
		t.Errorf("esperava a recusa pelo aninhamento; veio %v", erros)
	}
}

// Everything the validator does not implement makes the schema fail to load,
// so that a future version is not checked with a rule silently missing.
func TestConstrucoesNaoSuportadas(t *testing.T) {
	const cabecalho = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:t" xmlns="urn:t" elementFormDefault="qualified"`
	casos := map[string]string{
		"attributeFormDefault qualified": cabecalho + ` attributeFormDefault="qualified"><xs:element name="r" type="xs:string"/></xs:schema>`,
		"xs:group":                       cabecalho + `><xs:group name="g"><xs:sequence/></xs:group></xs:schema>`,
		"xs:all":                         cabecalho + `><xs:complexType name="T"><xs:all/></xs:complexType></xs:schema>`,
		"complexContent":                 cabecalho + `><xs:complexType name="T"><xs:complexContent/></xs:complexType></xs:schema>`,
		"xs:list":                        cabecalho + `><xs:simpleType name="T"><xs:list itemType="xs:string"/></xs:simpleType></xs:schema>`,
		"faceta totalDigits":             cabecalho + `><xs:simpleType name="T"><xs:restriction base="xs:string"><xs:totalDigits value="3"/></xs:restriction></xs:simpleType></xs:schema>`,
		"elemento nillable":              cabecalho + `><xs:element name="r" type="xs:string" nillable="true"/></xs:schema>`,
		"elemento com form":              cabecalho + `><xs:element name="r" type="xs:string" form="qualified"/></xs:schema>`,
		"atributo com default":           cabecalho + `><xs:complexType name="T"><xs:attribute name="a" default="x"/></xs:complexType></xs:schema>`,
		"tipo primitivo desconhecido":    cabecalho + `><xs:element name="r" type="xs:duration"/></xs:schema>`,
		"tipo nao declarado":             cabecalho + `><xs:element name="r" type="Nada"/></xs:schema>`,
		"elemento sem tipo":              cabecalho + `><xs:element name="r"/></xs:schema>`,
		"padrao so do XSD":               cabecalho + `><xs:simpleType name="T"><xs:restriction base="xs:string"><xs:pattern value="\i\c*"/></xs:restriction></xs:simpleType></xs:schema>`,
		"include inexistente":            cabecalho + `><xs:include schemaLocation="outro.xsd"/></xs:schema>`,
	}
	for nome, xsd := range casos {
		t.Run(nome, func(t *testing.T) {
			if _, err := Carregar(fstest.MapFS{"s.xsd": {Data: []byte(xsd)}}, "s.xsd"); err == nil {
				t.Error("carregou")
			}
		})
	}
}
