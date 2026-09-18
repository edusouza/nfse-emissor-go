// Package docs holds tests that check the documentation against the code.
package docs

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/domain/query"
)

// longDigitRun matches any run of 20 or more digits, which in this repository's
// documentation is always meant to be an NFS-e access key.
var longDigitRun = regexp.MustCompile(`\b\d{20,}\b`)

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
