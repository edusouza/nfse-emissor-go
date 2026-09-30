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

func readAnnex(t *testing.T) map[string]rejection {
	t.Helper()
	list, err := readRejections(filepath.Join(repoRoot, filepath.FromSlash(annexI)))
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]rejection{}
	for i, r := range list {
		if !codePattern.MatchString(r.code) {
			t.Errorf("%q nao e um codigo", r.code)
		}
		if i > 0 && list[i-1].code >= r.code {
			t.Errorf("fora de ordem ou repetido: %s depois de %s", r.code, list[i-1].code)
		}
		byCode[r.code] = r
	}
	return byCode
}

func TestRejectionsFromAnnex(t *testing.T) {
	codes := readAnnex(t)

	e0600, ok := codes["E0600"]
	if !ok {
		t.Fatal("E0600 ausente; e a regra do MEI que informa aliquota")
	}
	if len(e0600.rules) != 1 {
		t.Fatalf("E0600: %d regras, quer 1", len(e0600.rules))
	}
	r := e0600.rules[0]
	if !strings.Contains(r.message, "MEI") {
		t.Errorf("E0600: mensagem %q", r.message)
	}
	// The row of E0600 has no path of its own: it comes from the cell merged
	// over the rules of pAliq.
	if r.field != "NFSe/infNFSe/DPS/infDPS/valores/trib/tribMun/pAliq" {
		t.Errorf("E0600: campo %q", r.field)
	}
	if r.level != "2" {
		t.Errorf("E0600: nivel %q", r.level)
	}

	// A reception rule, from the other sheet: no field, no level.
	e1200, ok := codes["E1200"]
	if !ok {
		t.Fatal("E1200 ausente; e a regra do certificado de transmissao")
	}
	if e1200.rules[0].field != "" || e1200.rules[0].message == "" {
		t.Errorf("E1200: %+v", e1200.rules[0])
	}

	// The annex gives E1570 to two different rules. Both are kept.
	if n := len(codes["E1570"].rules); n != 2 {
		t.Errorf("E1570: %d regras, quer 2", n)
	}

	// E0031 only runs on invoices a municipality shares with the national
	// repository; an emitter never receives it.
	if _, ok := codes["E0031"]; ok {
		t.Error("E0031 nao se aplica a recepcao da DPS e nao deveria aparecer")
	}

	for code, rej := range codes {
		for _, r := range rej.rules {
			if r.level != "" && r.field == "" {
				t.Errorf("%s: regra de campo sem caminho no XML; a mesclagem de celulas se perdeu", code)
			}
		}
	}
}

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
