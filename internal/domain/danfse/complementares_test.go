package danfse

import (
	"strings"
	"testing"
)

// The line of the Lei 12.741/2012 is mandatory even when the invoice states no
// estimate — indTotTrib = 0, which is what the example carries.
func TestComplementares_LinhaDeTributosSempreSai(t *testing.T) {
	doc, err := Parse(lerExemplo(t), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if !strings.Contains(doc.Complementares, "Lei nº 12.741/2012") {
		t.Errorf("faltou a linha de totais aproximados:\n%s", doc.Complementares)
	}
	if !strings.Contains(doc.Complementares, "Federais: - ; Estaduais: - ; Municipais: -") {
		t.Errorf("sem estimativa, as tres categorias deveriam sair com traco:\n%s", doc.Complementares)
	}
}

func TestComplementares_TotaisEmValores(t *testing.T) {
	const bloco = `<totTrib>
              <vTotTrib>
                <vTotTribFed>120.00</vTotTribFed>
                <vTotTribEst>0.00</vTotTribEst>
                <vTotTribMun>30.00</vTotTribMun>
              </vTotTrib>
            </totTrib>`

	conteudo := []byte(trocar(t, string(lerExemplo(t)),
		"<totTrib>\n              <indTotTrib>0</indTotTrib>\n            </totTrib>", bloco))

	doc, err := Parse(conteudo, nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if !strings.Contains(doc.Complementares, "Federais: R$ 120,00 ; Estaduais: R$ 0,00 ; Municipais: R$ 30,00") {
		t.Errorf("os totais em reais nao sairam como a NT pede:\n%s", doc.Complementares)
	}
}

func TestComplementares_TotaisEmPercentuais(t *testing.T) {
	const bloco = `<totTrib>
              <pTotTrib>
                <pTotTribFed>8.00</pTotTribFed>
                <pTotTribEst>0.00</pTotTribEst>
                <pTotTribMun>2.00</pTotTribMun>
              </pTotTrib>
            </totTrib>`

	conteudo := []byte(trocar(t, string(lerExemplo(t)),
		"<totTrib>\n              <indTotTrib>0</indTotTrib>\n            </totTrib>", bloco))

	doc, err := Parse(conteudo, nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if !strings.Contains(doc.Complementares, "Federais: 8,00% ; Estaduais: 0,00% ; Municipais: 2,00%") {
		t.Errorf("os totais em percentual nao sairam como a NT pede:\n%s", doc.Complementares)
	}
}

// The Simples Nacional states a single rate for everything it covers. Splitting
// it into three would invent a division the invoice never made.
func TestComplementares_TotaisDoSimplesNacional(t *testing.T) {
	conteudo := []byte(trocar(t, string(lerExemplo(t)),
		"<indTotTrib>0</indTotTrib>", "<pTotTribSN>6.00</pTotTribSN>"))

	doc, err := Parse(conteudo, nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if !strings.Contains(doc.Complementares, "Simples Nacional: 6,00%") {
		t.Errorf("o total do Simples Nacional nao saiu:\n%s", doc.Complementares)
	}
	if strings.Contains(doc.Complementares, "Estaduais") {
		t.Errorf("o Simples Nacional nao se divide em federais/estaduais/municipais:\n%s", doc.Complementares)
	}
}

// A replacement has to name the invoice it replaced — note 7.
func TestComplementares_NotaSubstituida(t *testing.T) {
	const chave = "41069022212345678000195000000000000125081234567890"
	const bloco = `<subst>
          <chSubstda>` + chave + `</chSubstda>
          <cMotivo>01</cMotivo>
        </subst>`

	conteudo := []byte(trocar(t, string(lerExemplo(t)), "<prest>", bloco+"\n        <prest>"))

	doc, err := Parse(conteudo, nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if !strings.Contains(doc.Complementares, "NFS-e Subst.: "+chave) {
		t.Errorf("a chave da nota substituida nao saiu:\n%s", doc.Complementares)
	}
}

// A prefix with nothing after it would say the invoice mentions something it
// does not.
func TestComplementares_PrefixoSemValorNaoAparece(t *testing.T) {
	doc, err := Parse(lerExemplo(t), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	for _, prefixo := range []string{prefixoObra, prefixoEvento, prefixoSubst, prefixoDocRef} {
		if strings.Contains(doc.Complementares, prefixo) {
			t.Errorf("%q apareceu sem valor por tras:\n%s", prefixo, doc.Complementares)
		}
	}
}

func TestCanhoto_NumeroEChave(t *testing.T) {
	doc, err := Parse(lerExemplo(t), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	const esperado = "126 / 41069022212345678000195000000000000126081234567890"
	if doc.Canhoto.Numero != esperado {
		t.Errorf("canhoto: esperava %q, veio %q", esperado, doc.Canhoto.Numero)
	}
}

// Nothing in the NFS-e says it was cancelled or replaced, so Parse never
// invents a watermark: whoever prints has to state it.
func TestParse_NaoInventaMarcaDagua(t *testing.T) {
	doc, err := Parse(lerExemplo(t), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Marca != SemMarca {
		t.Errorf("Parse marcou o documento como %q sem nada no XML dizer isso", doc.Marca)
	}
}
