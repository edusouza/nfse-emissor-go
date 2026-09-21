package municipio

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestTabelaEstaSincronizadaComOAnexo regenerates the embedded table from the
// government spreadsheet and compares it with what is committed.
//
// Everything else here tests the CSV against itself and would go on passing if
// a row had been lost on the way out of the .xlsx. Only the annex can say
// whether the table is right, and it is versioned in this repository.
func TestTabelaEstaSincronizadaComOAnexo(t *testing.T) {
	if testing.Short() {
		t.Skip("compila e roda o gerador; pulado em -short")
	}

	gerado := filepath.Join(t.TempDir(), "municipios.csv")

	cmd := exec.Command("go", "run", "gerar_lista.go", "-saida", gerado)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("o gerador falhou: %v\n%s", err, out)
	}

	novo, err := os.ReadFile(gerado)
	if err != nil {
		t.Fatal(err)
	}
	commitado, err := os.ReadFile("municipios.csv")
	if err != nil {
		t.Fatal(err)
	}

	if string(novo) != string(commitado) {
		t.Fatalf("municipios.csv nao e mais o que o anexo produz.\n" +
			"Se o anexo mudou, rode 'go generate ./internal/domain/municipio' e " +
			"registre a mudanca no CHANGELOG; o arquivo nao se edita a mao.")
	}
}
