package servico

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestListaEstaSincronizadaComOAnexo regenerates the embedded list from the
// government spreadsheet and compares it with what is committed.
//
// This is the check the package would otherwise not have. Everything else here
// tests the CSV against itself — that it parses, that the codes are well
// formed — and would go on passing if a row had been dropped on the way out of
// the .xlsx, or if someone had fixed a description by hand. Only the annex can
// say whether the list is right, and it is versioned in this repository, so
// asking it costs a `go run`.
func TestListaEstaSincronizadaComOAnexo(t *testing.T) {
	if testing.Short() {
		t.Skip("compila e roda o gerador; pulado em -short")
	}

	gerado := filepath.Join(t.TempDir(), "lista.csv")

	cmd := exec.Command("go", "run", "gerar_lista.go", "-saida", gerado)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("o gerador falhou: %v\n%s", err, out)
	}

	novo, err := os.ReadFile(gerado)
	if err != nil {
		t.Fatal(err)
	}
	commitado, err := os.ReadFile("lista.csv")
	if err != nil {
		t.Fatal(err)
	}

	if string(novo) != string(commitado) {
		t.Fatalf("lista.csv nao e mais o que o anexo produz.\n" +
			"Se o anexo mudou, rode 'go generate ./internal/domain/servico' e " +
			"registre a mudanca no CHANGELOG; o arquivo nao se edita a mao.")
	}
}
