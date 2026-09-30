package xmlbuilder

import (
	"strings"
	"testing"

	"github.com/beevik/etree"
)

func filhosDe(t *testing.T, xmlStr, p string) string {
	t.Helper()

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlStr); err != nil {
		t.Fatal(err)
	}
	el := doc.FindElement(p)
	if el == nil {
		t.Fatalf("%s nao existe", p)
	}
	var nomes []string
	for _, f := range el.ChildElements() {
		nomes = append(nomes, f.Tag)
	}
	return strings.Join(nomes, ",")
}

// TCEndereco is a choice of endNac or endExt, then the street. The builder
// used to write cMun, UF, CEP and cPais loose after the street.
func TestDPSBuilder_EnderecoNacionalDoTomador(t *testing.T) {
	cfg := basicDPSConfig()
	cfg.Taker = &DPSTaker{CNPJ: "11222333000181", Name: "CLIENTE SA", Address: &AddressConfig{
		Street: "Rua XV de Novembro", Number: "100", Complement: "Sala 2", Neighborhood: "Centro",
		MunicipalityCode: "4106902", State: "PR", PostalCode: "80020-310",
	}}
	xmlStr := buildXML(t, cfg)

	if got := filhosDe(t, xmlStr, "DPS/infDPS/toma/end"); got != "endNac,xLgr,nro,xCpl,xBairro" {
		t.Errorf("filhos de end = %s", got)
	}
	if got := filhosDe(t, xmlStr, "DPS/infDPS/toma/end/endNac"); got != "cMun,CEP" {
		t.Errorf("filhos de endNac = %s", got)
	}
	if got := path(t, xmlStr, "DPS/infDPS/toma/end/endNac/CEP"); got != "80020310" {
		t.Errorf("CEP = %q", got)
	}
}

func TestDPSBuilder_EnderecoEstrangeiroDoTomador(t *testing.T) {
	cfg := basicDPSConfig()
	cfg.Taker = &DPSTaker{NIF: "PT123456789", Name: "CLIENTE LDA", Address: &AddressConfig{
		Street: "Rua Augusta", Number: "100", Neighborhood: "Baixa",
		City: "Lisboa", State: "Lisboa", PostalCode: "1100-148", CountryCode: "pt",
	}}
	xmlStr := buildXML(t, cfg)

	if got := filhosDe(t, xmlStr, "DPS/infDPS/toma/end"); got != "endExt,xLgr,nro,xBairro" {
		t.Errorf("filhos de end = %s", got)
	}
	if got := filhosDe(t, xmlStr, "DPS/infDPS/toma/end/endExt"); got != "cPais,cEndPost,xCidade,xEstProvReg" {
		t.Errorf("filhos de endExt = %s", got)
	}
	for campo, quer := range map[string]string{"cPais": "PT", "cEndPost": "1100-148", "xCidade": "Lisboa", "xEstProvReg": "Lisboa"} {
		if got := path(t, xmlStr, "DPS/infDPS/toma/end/endExt/"+campo); got != quer {
			t.Errorf("%s = %q, esperava %q", campo, got, quer)
		}
	}
}

// "br" is Brazil too.
func TestAddressConfig_IsForeign(t *testing.T) {
	for pais, estrangeiro := range map[string]bool{"": false, "BR": false, "br": false, "PT": true, "pt": true} {
		if got := (&AddressConfig{CountryCode: pais}).IsForeign(); got != estrangeiro {
			t.Errorf("IsForeign(%q) = %v", pais, got)
		}
	}
}
