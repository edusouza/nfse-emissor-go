package servico

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestListaTemOsCodigosDoAnexo(t *testing.T) {
	todos := Todos()

	// The annex has 338 leaf codes (v1.01 added three to v1.00's 335). Asserting the count catches a generator
	// that silently dropped rows — the shape of failure this file already had
	// once, when a styled cell arrived split into runs and read as empty.
	if len(todos) != 338 {
		t.Fatalf("lista tem %d codigos, esperava 338", len(todos))
	}

	visto := make(map[string]bool, len(todos))
	anterior := ""
	for _, s := range todos {
		if len(s.Codigo) != 6 {
			t.Errorf("codigo %q nao tem 6 digitos", s.Codigo)
		}
		if _, err := strconv.Atoi(s.Codigo); err != nil {
			t.Errorf("codigo %q nao e numerico", s.Codigo)
		}
		if s.Descricao == "" {
			t.Errorf("codigo %s sem descricao", s.Codigo)
		}
		if s.Grupo == "" {
			t.Errorf("codigo %s sem grupo", s.Codigo)
		}
		if visto[s.Codigo] {
			t.Errorf("codigo %s aparece duas vezes", s.Codigo)
		}
		visto[s.Codigo] = true

		if s.Codigo <= anterior {
			t.Errorf("codigo %s vem depois de %s; a lista deveria estar ordenada", s.Codigo, anterior)
		}
		anterior = s.Codigo
	}
}

// TestCodigoEComposicaoDeItemSubitemDesdobro checks the rule the code is built
// from, rather than the values the generator happened to write: cTribNac is
// item, subitem and desdobro concatenated, two digits each.
func TestCodigoEComposicaoDeItemSubitemDesdobro(t *testing.T) {
	for _, s := range Todos() {
		item, err := strconv.Atoi(s.Item())
		if err != nil {
			t.Fatalf("%s: item %q nao e numero", s.Codigo, s.Item())
		}
		desdobro, err := strconv.Atoi(s.Desdobro())
		if err != nil {
			t.Fatalf("%s: desdobro %q nao e numero", s.Codigo, s.Desdobro())
		}
		subitem, err := strconv.Atoi(s.Codigo[2:4])
		if err != nil {
			t.Fatalf("%s: subitem invalido", s.Codigo)
		}

		if got := fmt.Sprintf("%02d%02d%02d", item, subitem, desdobro); got != s.Codigo {
			t.Errorf("item %d subitem %d desdobro %d compoem %s, mas o codigo e %s",
				item, subitem, desdobro, got, s.Codigo)
		}
		if item < 1 || subitem < 1 || desdobro < 1 {
			t.Errorf("%s: um codigo de folha nao pode ter zero em item, subitem ou desdobro", s.Codigo)
		}
	}
}

func TestSubitemUsaANotacaoDaLei(t *testing.T) {
	casos := map[string]struct{ item, subitem, desdobro string }{
		"010101": {"1", "1.01", "1"},
		"010302": {"1", "1.03", "2"},
		"171901": {"17", "17.19", "1"},
		"990101": {"99", "99.01", "1"},
	}

	for codigo, want := range casos {
		s, ok := PorCodigo(codigo)
		if !ok {
			t.Fatalf("%s nao esta na lista", codigo)
		}
		if s.Item() != want.item || s.Subitem() != want.subitem || s.Desdobro() != want.desdobro {
			t.Errorf("%s: item/subitem/desdobro = %s/%s/%s, esperava %s/%s/%s",
				codigo, s.Item(), s.Subitem(), s.Desdobro(), want.item, want.subitem, want.desdobro)
		}
	}
}

func TestPorCodigoAceitaComoAsPessoasEscrevem(t *testing.T) {
	// The leading zero is the trap: a spreadsheet shows 010101 as 10101, and
	// that is what gets copied into a terminal.
	for _, entrada := range []string{"010101", "10101", "1.01.01", " 010101 "} {
		s, ok := PorCodigo(entrada)
		if !ok {
			t.Fatalf("%q nao foi reconhecido", entrada)
		}
		if s.Codigo != "010101" {
			t.Errorf("%q virou %s", entrada, s.Codigo)
		}
	}

	for _, entrada := range []string{"", "999999", "0101", "abcdef"} {
		if _, ok := PorCodigo(entrada); ok {
			t.Errorf("%q nao deveria ser encontrado", entrada)
		}
	}
}

// TestAnexoCitadoExiste keeps the version this binary reports honest: the name
// is printed to the user as the source of the list, and a rename in
// docs/anexos/ would otherwise leave it pointing at nothing.
func TestAnexoCitadoExiste(t *testing.T) {
	caminho := filepath.Join("..", "..", "..", "docs", "anexos", Anexo)
	if _, err := os.Stat(caminho); err != nil {
		t.Fatalf("o anexo citado pela constante Anexo nao esta em docs/anexos: %v", err)
	}
}
