// Package docs holds tests that check the documentation against the code.
package docs

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/domain/query"
	"github.com/edusouza/nfse-emissor-go/internal/domain/servico"
)

// longDigitRun matches any run of 20 or more digits, which in this repository's
// documentation is always meant to be an NFS-e access key.
var longDigitRun = regexp.MustCompile(`\b\d{20,}\b`)

// serviceCodeInDocs matches a cTribNac where the documentation puts one: in
// the configuration field or after the flag. Matching bare six-digit runs
// instead would sweep up dates and amounts.
var serviceCodeInDocs = regexp.MustCompile(`(?:codigo_tributacao_nacional:\s*"?|--servico\s+)(\d{6})`)

// docFiles lists the documentation that shows commands a reader will copy.
var docFiles = []string{
	"README.md",
	"CHANGELOG.md",
	"exemplos/README.md",
	"exemplos/README-windows.md",
}

// TestAccessKeysInDocsAreValid guards against documenting a key that the tool
// then rejects.
//
// This is not hypothetical: the README shipped a hand-written example with 47
// digits, and the first person to copy it got "a chave de acesso deve ter
// exatamente 50 digitos". A reader who cannot trust a copy-pasteable example
// cannot trust the document.
func TestAccessKeysInDocsAreValid(t *testing.T) {
	root := repoRoot(t)

	for _, name := range docFiles {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatalf("nao foi possivel ler %s: %v", name, err)
			}

			for _, key := range longDigitRun.FindAllString(string(content), -1) {
				if err := query.ValidateAccessKey(key); err != nil {
					t.Errorf("%s documenta uma chave que o proprio nfse recusa:\n  %s\n  %v",
						name, key, err)
				}
			}
		})
	}
}

// TestServiceCodesInDocsExist keeps the examples from teaching a code the
// Sefin would reject.
//
// Six digits look right whatever they are: nothing in the emitter refuses a
// well-formed cTribNac, and a reader copying one out of the README would only
// find out at the rejection. The embedded list is the one thing that can tell.
func TestServiceCodesInDocsExist(t *testing.T) {
	root := repoRoot(t)

	// The configuration files count too: they are copied as starting points,
	// which makes a wrong code there worse than a wrong code in prose.
	arquivos := append(append([]string{}, docFiles...),
		"internal/config/exemplo.yaml",
		"exemplos/nfse.yaml",
		"exemplos/nota-consultoria.yaml",
	)

	for _, name := range arquivos {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatalf("nao foi possivel ler %s: %v", name, err)
			}

			for _, match := range serviceCodeInDocs.FindAllStringSubmatch(string(content), -1) {
				if _, ok := servico.PorCodigo(match[1]); !ok {
					t.Errorf("%s documenta o cTribNac %s, que nao esta na lista nacional (%s)",
						name, match[1], servico.Anexo)
				}
			}
		})
	}
}

// repoRoot walks up from the test's directory until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod nao encontrado a partir do diretorio do teste")
		}
		dir = parent
	}
}
