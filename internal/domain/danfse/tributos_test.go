package danfse

import (
	"strings"
	"testing"
)

func TestParse_Servico(t *testing.T) {
	doc, err := Parse(lerExemplo(t), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	// The six digits of the national code print punctuated, as the nota
	// técnica writes them: nn.nn.nn.
	if doc.Servico.CodigoTributacao != "01.07.01" {
		t.Errorf("codigo de tributacao: veio %q", doc.Servico.CodigoTributacao)
	}
	if doc.Servico.CodigoNBS != Traco {
		t.Errorf("codigo NBS ausente deveria virar traco, veio %q", doc.Servico.CodigoNBS)
	}
	if doc.Servico.Descricao != "Desenvolvimento de sistema de emissao de notas fiscais" {
		t.Errorf("descricao do servico: veio %q", doc.Servico.Descricao)
	}
	// With no municipal description, the national one takes the field.
	if doc.Servico.DescricaoCodigo != "Analise e desenvolvimento de sistemas" {
		t.Errorf("descricao do codigo: veio %q", doc.Servico.DescricaoCodigo)
	}
}

func TestParse_ISSQN(t *testing.T) {
	doc, err := Parse(lerExemplo(t), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	casos := []struct{ campo, obtido, esperado string }{
		{"tipo de tributacao", doc.ISSQN.TipoTributacao, "Operação tributável"},
		{"municipio de incidencia", doc.ISSQN.MunicipioIncidencia, "Curitiba"},
		{"regime especial", doc.ISSQN.RegimeEspecial, "Nenhum"},
		{"base de calculo", doc.ISSQN.BaseCalculo, "1.500,00"},
		{"aliquota", doc.ISSQN.Aliquota, "2,00%"},
		{"retencao", doc.ISSQN.Retencao, "Não Retido"},
		{"apurado", doc.ISSQN.Apurado, "30,00"},
		{"imunidade ausente", doc.ISSQN.TipoImunidade, Traco},
	}
	for _, caso := range casos {
		if caso.obtido != caso.esperado {
			t.Errorf("%s: esperava %q, veio %q", caso.campo, caso.esperado, caso.obtido)
		}
	}

	if !doc.ISSQN.Incide {
		t.Error("uma operacao tributavel tem de manter o bloco do ISSQN")
	}
}

// Note 4 of NT 008 replaces the whole municipal block with a sentence when the
// tax does not reach the operation — and only then: immunity and export are
// still inside the tax, and keep their own fields.
func TestParse_ISSQN_NaoIncidencia(t *testing.T) {
	casos := []struct {
		codigo string
		incide bool
	}{
		{"1", true}, // operação tributável
		{"2", true}, // imunidade
		{"3", true}, // exportação de serviço
		{"4", false},
	}

	for _, caso := range casos {
		conteudo := []byte(trocar(t, string(lerExemplo(t)),
			"<tribISSQN>1</tribISSQN>", "<tribISSQN>"+caso.codigo+"</tribISSQN>"))

		doc, err := Parse(conteudo, nil)
		if err != nil {
			t.Fatalf("Parse devolveu erro: %v", err)
		}
		if doc.ISSQN.Incide != caso.incide {
			t.Errorf("tribISSQN = %s: Incide deveria ser %v", caso.codigo, caso.incide)
		}
	}
}

// When PIS and COFINS were withheld, NT 008 moves them into the withheld total
// and zeroes the issuer's own debt. Printing both would count the same money
// twice.
func TestParse_Federal_PisCofinsRetidos(t *testing.T) {
	const bloco = `<tribFed>
            <piscofins>
              <CST>01</CST>
              <vPis>9.75</vPis>
              <vCofins>45.00</vCofins>
              <tpRetPisCofins>1</tpRetPisCofins>
            </piscofins>
            <vRetCSLL>15.00</vRetCSLL>
          </tribFed>`

	// The schema puts tribFed right after tribMun, and the fixture stays in
	// that order: a test that feeds an XML the XSD would reject stops being
	// evidence about real invoices.
	conteudo := []byte(trocar(t, string(lerExemplo(t)),
		"</tribMun>", "</tribMun>\n            "+bloco))

	doc, err := Parse(conteudo, nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Federal.ContribuicoesSociais != "69,75" {
		t.Errorf("contribuicoes sociais: esperava a soma 69,75, veio %q", doc.Federal.ContribuicoesSociais)
	}
	if doc.Federal.PIS != "0,00" || doc.Federal.COFINS != "0,00" {
		t.Errorf("PIS e COFINS retidos deveriam zerar no debito proprio, vieram %q e %q",
			doc.Federal.PIS, doc.Federal.COFINS)
	}
	if doc.Federal.DescricaoContribuicoes != "PIS/COFINS Retidos" {
		t.Errorf("descricao das contribuicoes: veio %q", doc.Federal.DescricaoContribuicoes)
	}
}

func TestParse_Federal_PisCofinsNaoRetidos(t *testing.T) {
	const bloco = `<tribFed>
            <piscofins>
              <CST>01</CST>
              <vPis>9.75</vPis>
              <vCofins>45.00</vCofins>
              <tpRetPisCofins>2</tpRetPisCofins>
            </piscofins>
            <vRetCSLL>15.00</vRetCSLL>
          </tribFed>`

	// The schema puts tribFed right after tribMun, and the fixture stays in
	// that order: a test that feeds an XML the XSD would reject stops being
	// evidence about real invoices.
	conteudo := []byte(trocar(t, string(lerExemplo(t)),
		"</tribMun>", "</tribMun>\n            "+bloco))

	doc, err := Parse(conteudo, nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Federal.ContribuicoesSociais != "15,00" {
		t.Errorf("sem retencao, as contribuicoes sociais sao so o CSLL: veio %q",
			doc.Federal.ContribuicoesSociais)
	}
	if doc.Federal.PIS != "9,75" || doc.Federal.COFINS != "45,00" {
		t.Errorf("PIS e COFINS deveriam sair como estao no XML, vieram %q e %q",
			doc.Federal.PIS, doc.Federal.COFINS)
	}
}

func TestParse_Totais(t *testing.T) {
	doc, err := Parse(lerExemplo(t), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Totais.ValorServico != "1.500,00" {
		t.Errorf("valor do servico: veio %q", doc.Totais.ValorServico)
	}
	if doc.Totais.ValorLiquido != "1.500,00" {
		t.Errorf("valor liquido: veio %q", doc.Totais.ValorLiquido)
	}
	if doc.Totais.TotalRetencoes != "0,00" {
		t.Errorf("total das retencoes: veio %q", doc.Totais.TotalRetencoes)
	}
	// A v1.00 invoice has no IBS/CBS to total.
	if doc.Totais.TotalIBSCBS != Traco || doc.Totais.ValorLiquidoComIBSCBS != Traco {
		t.Errorf("sem IBS/CBS os dois campos deveriam ser tracos, vieram %q e %q",
			doc.Totais.TotalIBSCBS, doc.Totais.ValorLiquidoComIBSCBS)
	}
}

// The whole IBS/CBS block is absent from layout v1.00, and note 12 says a field
// with nothing behind it prints a dash — not a zero, which would state a value
// the invoice never carried.
func TestParse_IBSCBS_AusenteViraTracos(t *testing.T) {
	doc, err := Parse(lerExemplo(t), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	campos := map[string]string{
		"CST":                   doc.IBSCBS.CST,
		"base de calculo":       doc.IBSCBS.BaseCalculo,
		"aliquota do IBS":       doc.IBSCBS.AliquotaIBS,
		"valor total do IBS":    doc.IBSCBS.ValorTotalIBS,
		"aliquota da CBS":       doc.IBSCBS.AliquotaCBS,
		"valor total da CBS":    doc.IBSCBS.ValorTotalCBS,
		"indicador de operacao": doc.IBSCBS.IndicadorOperacao,
	}
	for nome, valor := range campos {
		if valor != Traco {
			t.Errorf("%s: esperava %q, veio %q", nome, Traco, valor)
		}
	}
}

func TestMoeda(t *testing.T) {
	casos := []struct{ entrada, esperado string }{
		{"1500.00", "1.500,00"},
		{"0.50", "0,50"},
		{"1234567.89", "1.234.567,89"},
		{"30", "30,00"},
		{"0.5", "0,50"},
		{"-12.34", "-12,34"},
		{"", ""},
		// Something that is not a number is printed as it came: the invoice is
		// the government's, and hiding what it says is worse than showing it in
		// an unexpected shape.
		{"n/a", "n/a"},
	}

	for _, caso := range casos {
		if obtido := moeda(caso.entrada); obtido != caso.esperado {
			t.Errorf("moeda(%q) = %q, esperava %q", caso.entrada, obtido, caso.esperado)
		}
	}
}

func TestPercentual(t *testing.T) {
	casos := []struct{ entrada, esperado string }{
		{"2.00", "2,00%"},
		{"0.00", "0,00%"},
		{"12.5", "12,50%"},
		{"", ""},
	}

	for _, caso := range casos {
		if obtido := percentual(caso.entrada); obtido != caso.esperado {
			t.Errorf("percentual(%q) = %q, esperava %q", caso.entrada, obtido, caso.esperado)
		}
	}
}

func TestSomar(t *testing.T) {
	if soma := somar("15.00", "9.75", "45.00"); soma != "69.75" {
		t.Errorf("somar devolveu %q, esperava 69.75", soma)
	}
	// Nothing to add is not zero: the field has to fall to the dash of note 12.
	if soma := somar("", "", ""); soma != "" {
		t.Errorf("somar sem valores devolveu %q, esperava vazio", soma)
	}
	if soma := somar("0.10", "0.20"); soma != "0.30" {
		t.Errorf("somar devolveu %q, esperava 0.30 — centavos, nunca float", soma)
	}
}

func TestLimitar(t *testing.T) {
	longo := strings.Repeat("a", 50)

	if obtido := limitar(longo, 10); obtido != strings.Repeat("a", 7)+"..." {
		t.Errorf("limitar nao aplicou as reticencias: %q", obtido)
	}
	if obtido := limitar("curto", 10); obtido != "curto" {
		t.Errorf("limitar encurtou o que ja cabia: %q", obtido)
	}
	// Counting has to be in characters: "ação" is four characters and six
	// bytes, and a cut by byte could split an accented letter in half.
	if obtido := limitar("ação", 4); obtido != "ação" {
		t.Errorf("limitar contou bytes em vez de caracteres: %q", obtido)
	}
}
