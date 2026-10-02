package anexos

import (
	"strings"
	"testing"
)

func lerAnexo(t *testing.T, ler func(string) ([]Rejeicao, error), arquivo string) map[string]Rejeicao {
	t.Helper()
	list, err := ler(Caminho(arquivo))
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]Rejeicao{}
	for i, r := range list {
		if !PadraoCodigo.MatchString(r.Codigo) {
			t.Errorf("%q nao e um codigo", r.Codigo)
		}
		if i > 0 && list[i-1].Codigo >= r.Codigo {
			t.Errorf("fora de ordem ou repetido: %s depois de %s", r.Codigo, list[i-1].Codigo)
		}
		byCode[r.Codigo] = r
	}
	return byCode
}

func TestLerI(t *testing.T) {
	codes := lerAnexo(t, LerI, ArquivoI)

	e0600, ok := codes["E0600"]
	if !ok {
		t.Fatal("E0600 ausente; e a regra do MEI que informa aliquota")
	}
	if len(e0600.Regras) != 1 {
		t.Fatalf("E0600: %d regras, quer 1", len(e0600.Regras))
	}
	r := e0600.Regras[0]
	if !strings.Contains(r.Mensagem, "MEI") {
		t.Errorf("E0600: mensagem %q", r.Mensagem)
	}
	// The row of E0600 has no path of its own: it comes from the cell merged
	// over the rules of pAliq.
	if r.Campo != "NFSe/infNFSe/DPS/infDPS/valores/trib/tribMun/pAliq" {
		t.Errorf("E0600: campo %q", r.Campo)
	}
	if r.Nivel != "2" {
		t.Errorf("E0600: nivel %q", r.Nivel)
	}

	// A reception rule, from the other sheet: no field, no level.
	e1200, ok := codes["E1200"]
	if !ok {
		t.Fatal("E1200 ausente; e a regra do certificado de transmissao")
	}
	if e1200.Regras[0].Campo != "" || e1200.Regras[0].Mensagem == "" {
		t.Errorf("E1200: %+v", e1200.Regras[0])
	}

	// The annex gives E1570 to two different rules. Both are kept.
	if n := len(codes["E1570"].Regras); n != 2 {
		t.Errorf("E1570: %d regras, quer 2", n)
	}

	// E0031 only runs on invoices a municipality shares with the national
	// repository; an emitter never receives it.
	if _, ok := codes["E0031"]; ok {
		t.Error("E0031 nao se aplica a recepcao da DPS e nao deveria aparecer")
	}

	for code, rej := range codes {
		for _, r := range rej.Regras {
			if r.Nivel != "" && r.Campo == "" {
				t.Errorf("%s: regra de campo sem caminho no XML; a mesclagem de celulas se perdeu", code)
			}
		}
	}
}

func TestLerII(t *testing.T) {
	codes := lerAnexo(t, LerII, ArquivoII)

	// The refusal a late cancellation gets: a level 3 rule, because the
	// deadline is whatever the municipality set.
	e0822, ok := codes["E0822"]
	if !ok {
		t.Fatal("E0822 ausente; e o prazo de cancelamento")
	}
	r := e0822.Regras[0]
	if !strings.Contains(r.Mensagem, "prazo para o cancelamento") {
		t.Errorf("E0822: mensagem %q", r.Mensagem)
	}
	if r.Campo != "evento/pedRegEvento/infPedReg/chNFSe" {
		t.Errorf("E0822: campo %q", r.Campo)
	}
	if r.Nivel != "3" {
		t.Errorf("E0822: nivel %q", r.Nivel)
	}

	// The signature of the request the provider sends.
	if e1980, ok := codes["E1980"]; !ok || e1980.Regras[0].Campo != "evento/pedRegEvento/Signature" {
		t.Errorf("E1980: %+v", e1980)
	}

	// E2020 is the signature of an event a municipality shares; an emitter
	// never sends one.
	if _, ok := codes["E2020"]; ok {
		t.Error("E2020 nao se aplica ao pedido do prestador e nao deveria aparecer")
	}

	for code, rej := range codes {
		for _, r := range rej.Regras {
			if r.Campo == "" {
				t.Errorf("%s: regra sem caminho no XML; a mesclagem de celulas se perdeu", code)
			}
			if r.Nivel == "-" || r.Notas == "-" {
				t.Errorf("%s: o \"-\" do anexo deveria virar vazio: %+v", code, r)
			}
		}
	}
}

// The two annexes share a few codes. E1260 is the same rule, an expired
// layout version, about two different documents, and only the annex the
// answer came from says which field it is. That is why the tables are kept
// apart.
func TestAnexosSaoTabelasSeparadas(t *testing.T) {
	i := lerAnexo(t, LerI, ArquivoI)
	ii := lerAnexo(t, LerII, ArquivoII)
	if i["E1260"].Regras[0].Campo == ii["E1260"].Regras[0].Campo {
		t.Errorf("E1260 deveria apontar campos diferentes nos dois anexos: %q", i["E1260"].Regras[0].Campo)
	}
}
