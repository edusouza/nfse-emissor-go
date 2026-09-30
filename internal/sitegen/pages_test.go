package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteLinks(t *testing.T) {
	dirs := map[string]bool{"internal/cli": true, "exemplos": true}
	isDir := func(p string) bool { return dirs[p] }
	blob := repoURL + "/blob/master/"
	tree := repoURL + "/tree/master/"

	tests := []struct {
		name, from, to, in, want string
	}{
		{"adr para adr", "docs/decisoes/0001-a.md", "decisoes/0001-a.md",
			"ver [0002](0002-b.md).", "ver [0002](0002-b.md)."},
		{"ancora preservada", "docs/decisoes/0001-a.md", "decisoes/0001-a.md",
			"[x](0002-b.md#contexto)", "[x](0002-b.md#contexto)"},
		{"arquivo fora do site", "docs/decisoes/0001-a.md", "decisoes/0001-a.md",
			"[NT](../notas-tecnicas/nt.pdf)", "[NT](" + blob + "docs/notas-tecnicas/nt.pdf)"},
		{"diretorio vai para tree", "docs/decisoes/0001-a.md", "decisoes/0001-a.md",
			"[cli](../../internal/cli/)", "[cli](" + tree + "internal/cli)"},
		{"changelog para adr", "CHANGELOG.md", "changelog.md",
			"[ADR 0007](docs/decisoes/0007-c.md)", "[ADR 0007](decisoes/0007-c.md)"},
		{"adr para changelog", "docs/decisoes/0001-a.md", "decisoes/0001-a.md",
			"[log](../../CHANGELOG.md)", "[log](../changelog.md)"},
		{"indice das adrs", "docs/decisoes/README.md", "decisoes/index.md",
			"| [0001](0001-a.md) |", "| [0001](0001-a.md) |"},
		{"adr para o indice", "docs/decisoes/0001-a.md", "decisoes/0001-a.md",
			"[todas](README.md)", "[todas](index.md)"},
		{"readme nao e pagina", "CHANGELOG.md", "changelog.md",
			"[uso](README.md#uso)", "[uso](" + blob + "README.md#uso)"},
		{"absoluto intocado", "CHANGELOG.md", "changelog.md",
			"[#5](https://github.com/x/y/issues/5)", "[#5](https://github.com/x/y/issues/5)"},
		{"ancora local intocada", "CHANGELOG.md", "changelog.md",
			"[acima](#adicionado)", "[acima](#adicionado)"},
		{"mailto intocado", "CHANGELOG.md", "changelog.md",
			"[a](mailto:a@b.c)", "[a](mailto:a@b.c)"},
		{"titulo preservado", "CHANGELOG.md", "changelog.md",
			`[e](exemplos/ "exemplos")`, `[e](` + tree + `exemplos "exemplos")`},
		{"varios na linha", "CHANGELOG.md", "changelog.md",
			"[a](docs/decisoes/0001-a.md) e [b](go.mod)",
			"[a](decisoes/0001-a.md) e [b](" + blob + "go.mod)"},
		{"definicao de referencia", "CHANGELOG.md", "changelog.md",
			"[ref]: docs/decisoes/0001-a.md", "[ref]: decisoes/0001-a.md"},
		{"fora do repositorio intocado", "CHANGELOG.md", "changelog.md",
			"[x](../fora.md)", "[x](../fora.md)"},
		{"bloco de codigo intocado", "CHANGELOG.md", "changelog.md",
			"```\n[x](go.mod)\n```\n[y](go.mod)",
			"```\n[x](go.mod)\n```\n[y](" + blob + "go.mod)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rewriteLinks(tt.in, tt.from, tt.to, isDir); got != tt.want {
				t.Errorf("\n got %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestRun(t *testing.T) {
	out := t.TempDir()

	// A page left behind by an ADR renamed since the last run.
	stale := filepath.Join(out, decisionsPage, "0000-renomeada.md")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("velha"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run(repoRoot, out); err != nil {
		t.Fatal(err)
	}

	for _, page := range []string{rejectionsPage, changelogPage, "decisoes/index.md", "decisoes/0001-cli-em-vez-de-api.md"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(page))); err != nil {
			t.Errorf("%s nao foi gerada: %v", page, err)
		}
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("a pagina de uma ADR que nao existe mais continuou publicada")
	}

	adrs, _ := filepath.Glob(filepath.Join(repoRoot, filepath.FromSlash(decisionsDir), "*.md"))
	pages, _ := filepath.Glob(filepath.Join(out, decisionsPage, "*.md"))
	if len(pages) != len(adrs) {
		t.Errorf("%d paginas de decisao para %d arquivos", len(pages), len(adrs))
	}

	changelog, err := os.ReadFile(filepath.Join(out, changelogPage))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(changelog), "](docs/decisoes/") {
		t.Error("o changelog publicado ainda aponta para docs/decisoes/, que nao existe no site")
	}
}

func TestColumns(t *testing.T) {
	for _, tt := range []struct {
		name  string
		index int
	}{{"A", 1}, {"Z", 26}, {"AA", 27}, {"AB", 28}, {"AZ", 52}, {"BA", 53}} {
		if got := colIndex(tt.name); got != tt.index {
			t.Errorf("colIndex(%q) = %d, quer %d", tt.name, got, tt.index)
		}
		if got := colName(tt.index); got != tt.name {
			t.Errorf("colName(%d) = %q, quer %q", tt.index, got, tt.name)
		}
	}
}

func TestSpread(t *testing.T) {
	s := sheet{cells: map[string]string{"B4": "caminho/", "C4": "campo", "C5": "proprio"}}
	if err := s.spread("B4:C5"); err != nil {
		t.Fatal(err)
	}
	// Only the empty cells take the value of B4; the ones with their own
	// text keep it.
	want := map[string]string{"B4": "caminho/", "B5": "caminho/", "C4": "campo", "C5": "proprio"}
	for ref, v := range want {
		if s.cells[ref] != v {
			t.Errorf("%s = %q, quer %q", ref, s.cells[ref], v)
		}
	}
}
