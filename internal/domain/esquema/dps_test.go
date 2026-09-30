package esquema

import (
	"strings"
	"testing"
	"time"

	"github.com/edusouza/nfse-emissor-go/pkg/xmlbuilder"
)

// dpsDoEmissor builds a DPS the way the emitter does, so the schema is held
// against real output rather than hand-written fixtures — the fixtures were
// how the old validator's mistakes went unnoticed (issue #4).
func dpsDoEmissor(t testing.TB, ajustar func(*xmlbuilder.DPSConfig)) string {
	t.Helper()

	cfg := xmlbuilder.DPSConfig{
		Environment:        2,
		EmissionDateTime:   time.Date(2026, 9, 18, 10, 30, 0, 0, time.FixedZone("BRT", -3*3600)),
		ApplicationVersion: "nfse/v0.9.0",
		Series:             "00001",
		Number:             "123",
		CompetenceDate:     time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
		EmitterType:        1,
		MunicipalityCode:   "4106902",
		Provider: xmlbuilder.DPSProvider{
			CNPJ:      "12345678000195",
			Name:      "EMPRESA EXEMPLO LTDA",
			TaxRegime: "mei",
		},
		Service: xmlbuilder.DPSService{
			NationalCode:     "010101",
			Description:      "Desenvolvimento de sistemas sob encomenda",
			MunicipalityCode: "4106902",
		},
		Values: xmlbuilder.DPSValues{ServiceValue: 1500},
	}
	if ajustar != nil {
		ajustar(&cfg)
	}

	result, err := xmlbuilder.NewDPSBuilder(cfg).Build()
	if err != nil {
		t.Fatalf("Build falhou: %v", err)
	}
	return result.XML
}

func validarDPS(t *testing.T, xml string) []Erro {
	t.Helper()

	e, err := DPS()
	if err != nil {
		t.Fatal(err)
	}
	return e.Validar([]byte(xml))
}

func TestDPSDoEmissorPassaNoSchema(t *testing.T) {
	casos := map[string]func(*xmlbuilder.DPSConfig){
		"MEI sem tomador": nil,
		"ME/EPP com aliquota": func(c *xmlbuilder.DPSConfig) {
			c.Provider.TaxRegime = "me_epp"
			c.Values.ISSRate = 2.5
			c.Values.TotalTaxPercentSN = 6
		},
		"tomador com CNPJ e endereco": func(c *xmlbuilder.DPSConfig) {
			c.Taker = &xmlbuilder.DPSTaker{
				CNPJ: "11222333000181", Name: "CLIENTE SA", Email: "compras@cliente.com.br", Phone: "4133334444",
				Address: &xmlbuilder.AddressConfig{
					Street: "Rua XV de Novembro", Number: "100", Neighborhood: "Centro",
					MunicipalityCode: "4106902", State: "PR", PostalCode: "80020310",
				},
			}
		},
		"tomador no exterior": func(c *xmlbuilder.DPSConfig) {
			c.Taker = &xmlbuilder.DPSTaker{
				NIF: "PT123456789", Name: "CLIENTE LDA",
				Address: &xmlbuilder.AddressConfig{
					Street: "Rua Augusta", Number: "100", Neighborhood: "Baixa",
					City: "Lisboa", State: "Lisboa", PostalCode: "1100-148", CountryCode: "PT",
				},
			}
		},
		"tomador com CPF": func(c *xmlbuilder.DPSConfig) {
			c.Taker = &xmlbuilder.DPSTaker{CPF: "12345678909", Name: "FULANO DE TAL"}
		},
		"descontos e deducoes": func(c *xmlbuilder.DPSConfig) {
			c.Values.UnconditionalDiscount = 100
			c.Values.ConditionalDiscount = 50
			c.Values.Deductions = 200
		},
		"retencao pelo tomador": func(c *xmlbuilder.DPSConfig) {
			c.Taker = &xmlbuilder.DPSTaker{CNPJ: "11222333000181", Name: "CLIENTE SA"}
			c.Values.ISSRetention = 2
		},
		"substituicao": func(c *xmlbuilder.DPSConfig) {
			c.Substitution = &xmlbuilder.DPSSubstitution{
				AccessKey:  "41069022212345678000195000000000000126081234567890",
				ReasonCode: xmlbuilder.SubstReasonOther,
				ReasonText: "Valor do servico informado errado na nota original",
			}
		},
		"inscricao municipal": func(c *xmlbuilder.DPSConfig) {
			c.Provider.MunicipalRegistration = "1234567"
		},
	}

	for nome, ajustar := range casos {
		t.Run(nome, func(t *testing.T) {
			xml := dpsDoEmissor(t, ajustar)
			for _, e := range validarDPS(t, xml) {
				t.Errorf("%v", e)
			}
			if t.Failed() {
				t.Logf("DPS:\n%s", xml)
			}
		})
	}
}

// Each case breaks the emitter's own output in one way the Sefin rejects,
// and names the path the error must point at.
func TestDPSForaDoSchema(t *testing.T) {
	casos := []struct {
		nome       string
		de, para   string
		caminho    string
		fragmentos []string
	}{
		{
			nome: "ambiente fora da enumeracao", de: "<tpAmb>2</tpAmb>", para: "<tpAmb>3</tpAmb>",
			caminho: "/DPS/infDPS/tpAmb", fragmentos: []string{`"3"`, "1, 2"},
		},
		{
			nome: "CNPJ com letra", de: "<CNPJ>12345678000195</CNPJ>", para: "<CNPJ>1234567800019X</CNPJ>",
			caminho: "/DPS/infDPS/prest/CNPJ", fragmentos: []string{"TSCNPJ"},
		},
		{
			// TSString admits only Latin-1. The description is
			// TSStringComQuebraDeLinha, which admits anything, so the
			// rule is exercised on verAplic.
			nome: "travessao fora do Latin-1", de: "<verAplic>nfse/v0.9.0", para: "<verAplic>nfse—v0.9.0",
			caminho: "/DPS/infDPS/verAplic", fragmentos: []string{"Latin-1"},
		},
		{
			nome: "espaco no fim", de: "nfse/v0.9.0<", para: "nfse/v0.9.0 <",
			caminho: "/DPS/infDPS/verAplic", fragmentos: []string{"espaco"},
		},
		{
			nome: "descricao aceita qualquer caractere", de: "Desenvolvimento de sistemas", para: "Desenvolvimento — sistemas",
		},
		{
			nome: "deducao em valor e em percentual", de: "<trib>", para: "<vDedRed><pDR>10.00</pDR><vDR>150.00</vDR></vDedRed><trib>",
			caminho: "/DPS/infDPS/valores/vDedRed/vDR", fragmentos: []string{"completo"},
		},
		{
			nome: "tomador com CNPJ e CPF", de: "<serv>",
			para:    "<toma><CNPJ>11222333000181</CNPJ><CPF>12345678909</CPF><xNome>X</xNome></toma><serv>",
			caminho: "/DPS/infDPS/toma/CPF", fragmentos: []string{"xNome"},
		},
		{
			nome: "tomador depois do servico", de: "<valores>",
			para:    "<toma><CNPJ>11222333000181</CNPJ><xNome>X</xNome></toma><valores>",
			caminho: "/DPS/infDPS/toma", fragmentos: []string{"valores"},
		},
		{
			nome: "verAplic longo demais", de: "<verAplic>nfse/v0.9.0</verAplic>",
			para:    "<verAplic>v0.0.0-20260918103000-abcdef123456</verAplic>",
			caminho: "/DPS/infDPS/verAplic", fragmentos: []string{"no maximo 20"},
		},
		{
			nome: "elemento obrigatorio ausente", de: "<cLocEmi>4106902</cLocEmi>", para: "",
			caminho: "/DPS/infDPS/prest", fragmentos: []string{"cLocEmi"},
		},
		{
			nome: "elemento desconhecido", de: "<cLocEmi>4106902</cLocEmi>", para: "<cLocEmi>4106902</cLocEmi><extra>1</extra>",
			caminho: "/DPS/infDPS/extra", fragmentos: []string{"prest"},
		},
		{
			nome: "atributo obrigatorio ausente", de: `versao="1.00"`, para: "",
			caminho: "/DPS", fragmentos: []string{"versao"},
		},
		{
			nome: "versao que o schema nao conhece", de: `versao="1.00"`, para: `versao="2.00"`,
			caminho: "/DPS/@versao", fragmentos: []string{"TVerNFSe"},
		},
		{
			nome: "atributo nao declarado", de: `versao="1.00"`, para: `versao="1.00" extra="1"`,
			caminho: "/DPS/@extra", fragmentos: []string{"nao permitido"},
		},
		{
			nome: "data de competencia inexistente", de: "<dCompet>2026-09-18</dCompet>", para: "<dCompet>2026-02-30</dCompet>",
			caminho: "/DPS/infDPS/dCompet",
		},
		{
			nome: "texto em conteudo de elementos", de: "<prest>", para: "<prest>texto",
			caminho: "/DPS/infDPS/prest", fragmentos: []string{"texto"},
		},
	}

	base := dpsDoEmissor(t, nil)
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if !strings.Contains(base, c.de) {
				t.Fatalf("a DPS de base nao contem %q:\n%s", c.de, base)
			}
			erros := validarDPS(t, strings.Replace(base, c.de, c.para, 1))
			if c.caminho == "" {
				for _, e := range erros {
					t.Errorf("recusou o que o schema aceita: %v", e)
				}
				return
			}
			if len(erros) == 0 {
				t.Fatal("o schema aceitou")
			}
			e := erros[0]
			if e.Caminho != c.caminho {
				t.Errorf("caminho = %q, esperava %q (%v)", e.Caminho, c.caminho, e)
			}
			for _, f := range c.fragmentos {
				if !strings.Contains(e.Mensagem, f) {
					t.Errorf("a mensagem nao traz %q: %v", f, e)
				}
			}
		})
	}
}

func TestDocumentosQueNaoSaoDPS(t *testing.T) {
	casos := map[string]string{
		"XML malformado":    "<DPS",
		"raiz desconhecida": `<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.00"/>`,
		"namespace errado":  `<DPS xmlns="urn:outro" versao="1.00"/>`,
		"sem namespace":     `<DPS versao="1.00"/>`,
		"vazio":             "",
	}
	for nome, doc := range casos {
		t.Run(nome, func(t *testing.T) {
			if erros := validarDPS(t, doc); len(erros) == 0 {
				t.Error("aceitou")
			}
		})
	}
}

// Carried over from the hand-written validator's tests: a cancellation code
// ("1") is a non-empty string and would pass a presence check; only the
// TSCodJustSubst enumeration refuses it. The key must be 50 digits.
func TestSubstForaDoSchema(t *testing.T) {
	base := dpsDoEmissor(t, func(c *xmlbuilder.DPSConfig) {
		c.Substitution = &xmlbuilder.DPSSubstitution{
			AccessKey:  "41069022212345678000195000000000000126081234567890",
			ReasonCode: xmlbuilder.SubstReasonOther,
			ReasonText: "Valor do servico informado errado na nota original",
		}
	})

	casos := map[string]struct{ de, para, caminho string }{
		"codigo de cancelamento": {"<cMotivo>99</cMotivo>", "<cMotivo>1</cMotivo>", "/DPS/infDPS/subst/cMotivo"},
		"chave curta":            {"41069022212345678000195000000000000126081234567890", "123", "/DPS/infDPS/subst/chSubstda"},
		"chave com prefixo":      {"<chSubstda>", "<chSubstda>NFS", "/DPS/infDPS/subst/chSubstda"},
		"chave longa":            {"</chSubstda>", "9</chSubstda>", "/DPS/infDPS/subst/chSubstda"},
		"motivo curto":           {"Valor do servico informado errado na nota original", "curto", "/DPS/infDPS/subst/xMotivo"},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			if !strings.Contains(base, c.de) {
				t.Fatalf("a DPS de base nao contem %q", c.de)
			}
			erros := validarDPS(t, strings.Replace(base, c.de, c.para, 1))
			if len(erros) == 0 || erros[0].Caminho != c.caminho {
				t.Errorf("esperava erro em %s; veio %v", c.caminho, erros)
			}
		})
	}
}
