package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestAvisarIBSCBS(t *testing.T) {
	casos := map[string]bool{
		"2026-12-31": false,
		"2027-01-01": true,
		"2028-06-15": true,
	}
	for data, avisa := range casos {
		competencia, _ := time.Parse("2006-01-02", data)
		var out bytes.Buffer
		avisarIBSCBS(&out, competencia)
		if got := strings.Contains(out.String(), "IBSCBS"); got != avisa {
			t.Errorf("competencia %s: aviso=%v, esperava %v\n%s", data, got, avisa, out.String())
		}
	}
}

// The warning reaches the user on an ordinary emission, and does not stop it.
func TestEmitir_AvisaIBSCBSEm2027(t *testing.T) {
	dir := workspace(t)

	out, err := runEmit(t, dir, "--numero", "11", "--valor", "1500", "--descricao", "Consultoria",
		"--competencia", "2027-01-15")
	if err != nil {
		t.Fatalf("emitir falhou: %v\n%s", err, out)
	}
	if !strings.Contains(out, "grupo IBSCBS") || !strings.Contains(out, "issues/21") {
		t.Errorf("o aviso nao apareceu:\n%s", out)
	}

	out, err = runEmit(t, dir, "--numero", "12", "--valor", "1500", "--descricao", "Consultoria",
		"--competencia", "2026-09-15")
	if err != nil {
		t.Fatalf("emitir falhou: %v\n%s", err, out)
	}
	if strings.Contains(out, "grupo IBSCBS") {
		t.Errorf("avisou para uma competencia de 2026:\n%s", out)
	}
}
