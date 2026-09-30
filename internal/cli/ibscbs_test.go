package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAvisarIBSCBS(t *testing.T) {
	brasilia := time.FixedZone("BRT", -3*3600)
	casos := []struct {
		competencia time.Time
		avisa       bool
	}{
		{time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), false},
		// 22:00 in Brasília is 01:00 of 2027 in UTC; the invoice is 2026's.
		{time.Date(2026, 12, 31, 22, 0, 0, 0, brasilia), false},
		{time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), true},
		{time.Date(2027, 1, 1, 0, 30, 0, 0, brasilia), true},
		{time.Date(2028, 6, 15, 0, 0, 0, 0, time.UTC), true},
	}
	for _, c := range casos {
		var out bytes.Buffer
		avisarIBSCBS(&out, c.competencia)
		if got := strings.Contains(out.String(), "grupo IBSCBS"); got != c.avisa {
			t.Errorf("competencia %v: aviso=%v, esperava %v\n%s", c.competencia, got, c.avisa, out.String())
		}
		if c.avisa && !strings.Contains(out.String(), c.competencia.Format("02/01/2006")) {
			t.Errorf("o aviso nao cita a competencia %s:\n%s", c.competencia.Format("02/01/2006"), out.String())
		}
	}
}

// The warning goes to stderr, keeping stdout clean, and the invoice is still
// produced.
func TestEmitir_AvisoIBSCBSVaiParaOStderr(t *testing.T) {
	dir := workspace(t)
	t.Setenv(envCertPassword, testCertPassword)

	var stdout, stderr bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"emitir", "--config", filepath.Join(dir, "nfse.yaml"),
		"--numero", "13", "--valor", "1500", "--descricao", "Consultoria", "--competencia", "2027-02-01"})
	if err := root.Execute(); err != nil {
		t.Fatalf("emitir falhou: %v\n%s%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "grupo IBSCBS") {
		t.Errorf("o aviso nao foi para o stderr:\n%s", stderr.String())
	}
	if strings.Contains(stdout.String(), "grupo IBSCBS") {
		t.Errorf("o aviso foi para o stdout:\n%s", stdout.String())
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
