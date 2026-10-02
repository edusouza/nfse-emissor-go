package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot is the repository, seen from this package's directory, which is
// where go test runs.
const repoRoot = "../.."

func TestLocalCodesAreMarked(t *testing.T) {
	local, err := localCodes(filepath.Join(repoRoot, "internal", "domain", "validation"))
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"E0595", "E0600", "E0621", "E0625"} {
		if _, ok := local[code]; !ok {
			t.Errorf("%s: a validacao local o confere, mas nenhuma mensagem o cita", code)
		}
	}

	out := t.TempDir()
	if err := writeRejections(repoRoot, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rejectionsPage)))
	if err != nil {
		t.Fatal(err)
	}
	page := string(data)
	section := func(code string) string {
		_, after, ok := strings.Cut(page, "\n## "+code+"\n")
		if !ok {
			t.Fatalf("%s sem secao na pagina", code)
		}
		s, _, _ := strings.Cut(after, "\n## ")
		return s
	}
	if !strings.Contains(section("E0600"), "Conferido pelo `nfse`") {
		t.Error("E0600 deveria vir marcado como conferido localmente")
	}
	if strings.Contains(section("E0001"), "Conferido pelo `nfse`") {
		t.Error("E0001 nao e conferido localmente")
	}

	// The annex wraps the message of E0312 after "DPS, "; the page must not.
	if !strings.Contains(section("E0312"), "informada na DPS, conforme a lista") {
		t.Errorf("a mensagem do E0312 deveria sair numa linha so:\n%s", section("E0312"))
	}
}

// The cancellation's codes come from ANEXO II, on a page of their own: the
// annexes share a few numbers, and one page would mix two meanings.
func TestEventRejectionsPage(t *testing.T) {
	out := t.TempDir()
	if err := writeRejections(repoRoot, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(eventRejectionsPage)))
	if err != nil {
		t.Fatal(err)
	}
	page := string(data)
	_, after, ok := strings.Cut(page, "\n## E0822\n")
	if !ok {
		t.Fatal("E0822, o prazo de cancelamento, sem secao na pagina de eventos")
	}
	section, _, _ := strings.Cut(after, "\n## ")
	for _, want := range []string{"prazo para o cancelamento", "evento/pedRegEvento/infPedReg/chNFSe", "Nível 3"} {
		if !strings.Contains(section, want) {
			t.Errorf("E0822: faltou %q:\n%s", want, section)
		}
	}
	if strings.Contains(page, "\n## E0600\n") {
		t.Error("E0600 e da DPS e nao deveria estar na pagina de eventos")
	}
	if strings.Contains(page, "Conferido pelo `nfse`") {
		t.Error("nenhum codigo de evento e conferido localmente")
	}
	if !strings.HasPrefix(strings.SplitN(page, "\n", 7)[5], generatedMarker) {
		t.Error("a pagina de eventos deveria trazer o marcador de pagina gerada")
	}
}

func TestInline(t *testing.T) {
	tests := []struct{ in, want string }{
		{"texto comum", "texto comum"},
		{"- item", `\- item`},
		{"+ item", `\+ item`},
		{"# titulo", `\# titulo`},
		{"> citacao", `\> citacao`},
		{"12. passo", `12\. passo`},
		{"valor 1.5", "valor 1.5"},
		{"a*b_c", `a\*b\_c`},
		{"[x](y)", `\[x\](y)`},
		{"<tag>", "&lt;tag>"},
		{"`codigo`", "\\`codigo\\`"},
		{`C:\pasta`, `C:\\pasta`},
	}
	for _, tt := range tests {
		if got := inline(tt.in); got != tt.want {
			t.Errorf("inline(%q) = %q, quer %q", tt.in, got, tt.want)
		}
	}
}

func TestProse(t *testing.T) {
	in := "Primeira linha\nsegunda linha\n\n- item\r\n\n\n  Terceiro  "
	want := "Primeira linha  \nsegunda linha\n\n\\- item\n\nTerceiro"
	if got := prose(in); got != want {
		t.Errorf("prose:\n%q\nquer\n%q", got, want)
	}
}
