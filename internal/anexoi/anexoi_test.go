package anexoi

import (
	"strings"
	"testing"
)

func lerAnexo(t *testing.T) map[string]Rejeicao {
	t.Helper()
	list, err := Ler(Caminho())
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

func TestLer(t *testing.T) {
	codes := lerAnexo(t)

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
