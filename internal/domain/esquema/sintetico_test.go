package esquema

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// schemaCompleto loads a synthetic schema written in full.
func schemaCompleto(t *testing.T, corpo string) *Esquema {
	t.Helper()

	xsd := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:t" xmlns="urn:t" elementFormDefault="qualified">` +
		corpo + `</xs:schema>`
	e, err := Carregar(fstest.MapFS{"s.xsd": {Data: []byte(xsd)}}, "s.xsd")
	if err != nil {
		t.Fatalf("schema sintetico nao carregou: %v", err)
	}
	return e
}

func confere(t *testing.T, e *Esquema, doc string, valido bool, fragmento string) {
	t.Helper()

	erros := e.Validar([]byte(doc))
	switch {
	case valido && len(erros) > 0:
		t.Errorf("recusou %s: %v", doc, erros)
	case !valido && len(erros) == 0:
		t.Errorf("aceitou %s", doc)
	case !valido && fragmento != "" && !strings.Contains(erros[0].Error(), fragmento):
		t.Errorf("%s: esperava %q; veio %v", doc, fragmento, erros[0])
	}
}

func TestFacetas(t *testing.T) {
	e := schemaCompleto(t, `
  <xs:simpleType name="Base"><xs:restriction base="xs:string"><xs:pattern value="[a-z]+"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Alternativas"><xs:restriction base="xs:string"><xs:pattern value="a+"/><xs:pattern value="b+"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Derivado"><xs:restriction base="Base"><xs:pattern value="x.*"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Exato"><xs:restriction base="xs:string"><xs:length value="3"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Minimo"><xs:restriction base="xs:string"><xs:minLength value="15"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Colapsa"><xs:restriction base="xs:string"><xs:whiteSpace value="collapse"/><xs:pattern value="[a-z]+( [a-z]+)*"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Enum"><xs:restriction base="xs:string">
    <xs:enumeration value="1"/><xs:enumeration value="2"/><xs:enumeration value="3"/><xs:enumeration value="4"/>
    <xs:enumeration value="5"/><xs:enumeration value="6"/><xs:enumeration value="7"/><xs:enumeration value="8"/>
    <xs:enumeration value="9"/><xs:enumeration value="10"/><xs:enumeration value="11"/><xs:enumeration value="12"/>
    <xs:enumeration value="13"/><xs:enumeration value="14"/></xs:restriction></xs:simpleType>
  <xs:element name="alt" type="Alternativas"/>
  <xs:element name="der" type="Derivado"/>
  <xs:element name="exato" type="Exato"/>
  <xs:element name="minimo" type="Minimo"/>
  <xs:element name="colapsa" type="Colapsa"/>
  <xs:element name="inteiro" type="xs:integer"/>
  <xs:element name="decimal" type="xs:decimal"/>
  <xs:element name="enum" type="Enum"/>`)

	casos := []struct {
		doc    string
		valido bool
		frag   string
	}{
		// Patterns at one level are alternatives.
		{`<alt xmlns="urn:t">aaa</alt>`, true, ""},
		{`<alt xmlns="urn:t">bb</alt>`, true, ""},
		{`<alt xmlns="urn:t">ab</alt>`, false, "Alternativas"},
		// Patterns at different levels must all hold.
		{`<der xmlns="urn:t">xyz</der>`, true, ""},
		{`<der xmlns="urn:t">abc</der>`, false, "Derivado"},
		{`<der xmlns="urn:t">x1</der>`, false, "Base"},
		// length counts characters, not bytes.
		{`<exato xmlns="urn:t">açú</exato>`, true, ""},
		{`<exato xmlns="urn:t">ab</exato>`, false, "exatamente 3"},
		// minLength boundary.
		{`<minimo xmlns="urn:t">` + strings.Repeat("x", 15) + `</minimo>`, true, ""},
		{`<minimo xmlns="urn:t">` + strings.Repeat("x", 14) + `</minimo>`, false, "no minimo 15"},
		// collapse normalizes before the pattern.
		{"<colapsa xmlns=\"urn:t\"> abc \t def\n</colapsa>", true, ""},
		// xs:integer collapses, and is checked lexically.
		{"<inteiro xmlns=\"urn:t\"> 42\n</inteiro>", true, ""},
		{`<inteiro xmlns="urn:t">12a</inteiro>`, false, "xs:integer"},
		{`<decimal xmlns="urn:t">-1.50</decimal>`, true, ""},
		{`<decimal xmlns="urn:t">1,50</decimal>`, false, "xs:decimal"},
		// A long enumeration is cut short in the message.
		{`<enum xmlns="urn:t">99</enum>`, false, "e mais 2"},
	}
	for _, c := range casos {
		confere(t, e, c.doc, c.valido, c.frag)
	}
}

func TestConteudo(t *testing.T) {
	e := schemaCompleto(t, `
  <xs:simpleType name="S"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:complexType name="Vazio"/>
  <xs:complexType name="ComTexto"><xs:simpleContent><xs:extension base="xs:string"><xs:attribute name="id" type="xs:ID"/></xs:extension></xs:simpleContent></xs:complexType>
  <xs:complexType name="Misto" mixed="true"><xs:sequence><xs:element name="b" type="S" minOccurs="0"/></xs:sequence></xs:complexType>
  <xs:complexType name="Lista"><xs:sequence><xs:element name="i" type="Item" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
  <xs:simpleType name="Item"><xs:restriction base="xs:string"><xs:pattern value="[0-9]+"/></xs:restriction></xs:simpleType>
  <xs:complexType name="No"><xs:sequence><xs:element name="n" type="No" minOccurs="0"/><xs:element name="v" type="S"/></xs:sequence></xs:complexType>
  <xs:element name="simples" type="S"/>
  <xs:element name="vazio" type="Vazio"/>
  <xs:element name="texto" type="ComTexto"/>
  <xs:element name="misto" type="Misto"/>
  <xs:element name="lista" type="Lista"/>
  <xs:element name="no" type="No"/>`)

	casos := []struct {
		doc    string
		valido bool
		frag   string
	}{
		{`<simples xmlns="urn:t">x<b/></simples>`, false, "nao pode conter elementos"},
		{`<simples xmlns="urn:t" x="1">x</simples>`, false, "/simples/@x"},
		{`<vazio xmlns="urn:t"><b/></vazio>`, false, "/vazio/b"},
		{`<vazio xmlns="urn:t"/>`, true, ""},
		{`<texto xmlns="urn:t" id="a1">abc</texto>`, true, ""},
		{`<texto xmlns="urn:t" id="1a">abc</texto>`, false, "xs:ID"},
		{`<texto xmlns="urn:t">abc<b/></texto>`, false, "so texto"},
		{`<misto xmlns="urn:t">livre<b>x</b>texto</misto>`, true, ""},
		// The path of a repeated element carries its position.
		{`<lista xmlns="urn:t"><i>1</i><i>x</i></lista>`, false, "/lista/i[2]"},
		// A recursive type is checked at the depth where it breaks.
		{`<no xmlns="urn:t"><n><n><v>a</v></n><v>b</v></n></no>`, false, "/no: falta v"},
		{`<no xmlns="urn:t"><n><n><v>a</v></n><v>b</v></n><v>c</v></no>`, true, ""},
	}
	for _, c := range casos {
		confere(t, e, c.doc, c.valido, c.frag)
	}
}

// The three ways a wildcard treats what it matches. The XML-DSig schema uses
// lax and strict; skip is checked here so that it is not left to chance.
func TestCuringas(t *testing.T) {
	e := schemaCompleto(t, `
  <xs:simpleType name="Digitos"><xs:restriction base="xs:string"><xs:pattern value="[0-9]+"/></xs:restriction></xs:simpleType>
  <xs:element name="d" type="Digitos"/>
  <xs:element name="lax"><xs:complexType><xs:sequence><xs:any namespace="##any" processContents="lax" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element>
  <xs:element name="strict"><xs:complexType><xs:sequence><xs:any namespace="##any" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element>
  <xs:element name="skip"><xs:complexType><xs:sequence><xs:any namespace="##any" processContents="skip" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element>
  <xs:element name="outro"><xs:complexType><xs:sequence><xs:any namespace="##other" processContents="skip"/></xs:sequence></xs:complexType></xs:element>`)

	casos := []struct {
		doc    string
		valido bool
		frag   string
	}{
		// lax: validates what is declared, lets through what is not.
		{`<lax xmlns="urn:t"><d>x</d></lax>`, false, "Digitos"},
		{`<lax xmlns="urn:t"><d>1</d><f xmlns="urn:x"/></lax>`, true, ""},
		// strict: requires a declaration.
		{`<strict xmlns="urn:t"><f xmlns="urn:x"/></strict>`, false, "nao e declarado"},
		{`<strict xmlns="urn:t"><d>1</d></strict>`, true, ""},
		// skip: looks at nothing inside.
		{`<skip xmlns="urn:t"><d>x</d></skip>`, true, ""},
		// ##other: another namespace, never none and never the target.
		{`<outro xmlns="urn:t"><f xmlns="urn:x"/></outro>`, true, ""},
		{`<outro xmlns="urn:t"><f xmlns=""/></outro>`, false, ""},
		{`<outro xmlns="urn:t"><d>1</d></outro>`, false, ""},
	}
	for _, c := range casos {
		confere(t, e, c.doc, c.valido, c.frag)
	}
}

// Nested unbounded groups used to make the matcher revisit the same
// positions over and over.
func TestCasamentoNaoExplode(t *testing.T) {
	e := schemaCompleto(t, `
  <xs:simpleType name="V"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:element name="r"><xs:complexType><xs:sequence maxOccurs="unbounded"><xs:choice>
    <xs:element name="a" type="V" minOccurs="0" maxOccurs="unbounded"/><xs:element name="b" type="V" minOccurs="0"/>
  </xs:choice></xs:sequence></xs:complexType></xs:element>`)

	doc := `<r xmlns="urn:t">` + strings.Repeat(`<a/><b/>`, 800) + `</r>`
	inicio := time.Now()
	if erros := e.Validar([]byte(doc)); len(erros) > 0 {
		t.Fatalf("recusou: %v", erros[0])
	}
	if d := time.Since(inicio); d > 2*time.Second {
		t.Errorf("1600 filhos levaram %v", d)
	}
}

func TestConstrucoesIgnoradasAgoraSaoRecusadas(t *testing.T) {
	const cab = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:t" xmlns="urn:t" elementFormDefault="qualified">`
	casos := map[string]string{
		"xs:key":                    `<xs:element name="r" type="xs:string"><xs:key name="k"><xs:selector xpath="."/><xs:field xpath="."/></xs:key></xs:element>`,
		"use prohibited":            `<xs:complexType name="T"><xs:attribute name="a" type="xs:string" use="prohibited"/></xs:complexType>`,
		"processContents invalido":  `<xs:complexType name="T"><xs:sequence><xs:any processContents="bogus"/></xs:sequence></xs:complexType>`,
		"namespace ##bogus":         `<xs:complexType name="T"><xs:sequence><xs:any namespace="##bogus"/></xs:sequence></xs:complexType>`,
		"type e tipo inline":        `<xs:element name="r" type="xs:string"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:element>`,
		"sequence e simpleContent":  `<xs:complexType name="T"><xs:sequence/><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>`,
		"atributo sem nome":         `<xs:complexType name="T"><xs:attribute type="xs:string"/></xs:complexType>`,
		"elemento local sem nome":   `<xs:complexType name="T"><xs:sequence><xs:element type="xs:string"/></xs:sequence></xs:complexType>`,
		"declaracao duplicada":      `<xs:element name="r" type="xs:string"/><xs:element name="r" type="xs:string"/>`,
		"maxOccurs menor que o min": `<xs:complexType name="T"><xs:sequence><xs:element name="a" type="xs:string" minOccurs="2" maxOccurs="1"/></xs:sequence></xs:complexType>`,
		"whiteSpace invalido":       `<xs:simpleType name="T"><xs:restriction base="xs:string"><xs:whiteSpace value="trim"/></xs:restriction></xs:simpleType>`,
		"filho estranho do schema":  `<outro/>`,
		"attributeGroup":            `<xs:attributeGroup name="g"/>`,
		"simpleContent restriction": `<xs:complexType name="T"><xs:simpleContent><xs:restriction base="xs:string"/></xs:simpleContent></xs:complexType>`,
	}
	for nome, corpo := range casos {
		t.Run(nome, func(t *testing.T) {
			xsd := cab + corpo + `</xs:schema>`
			if _, err := Carregar(fstest.MapFS{"s.xsd": {Data: []byte(xsd)}}, "s.xsd"); err == nil {
				t.Error("carregou")
			}
		})
	}
}

func TestSerieDPS(t *testing.T) {
	base := dpsDoEmissor(t, nil)
	for serie, valida := range map[string]bool{"00001": true, "1": true, "12345": true, "A1": false, "0000000001": false, "": false} {
		doc := strings.Replace(base, "<serie>00001</serie>", "<serie>"+serie+"</serie>", 1)
		if erros := validarDPS(t, doc); (len(erros) == 0) != valida {
			t.Errorf("serie %q: valida=%v; erros %v", serie, !valida, erros)
		}
	}
}
