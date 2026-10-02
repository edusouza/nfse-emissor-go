package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/config"
)

// skipWithoutUnixModes skips where file modes are not Unix permissions.
func skipWithoutUnixModes(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("o Windows nao tem as permissoes de arquivo do Unix")
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s com permissao %#o, esperava %#o", filepath.Base(path), got, want)
	}
}

func TestWriteNew_CreatesOwnerOnly(t *testing.T) {
	skipWithoutUnixModes(t)
	path := filepath.Join(t.TempDir(), "nota-nfse.xml")

	if err := writeNew(path, []byte("<NFSe/>"), false); err != nil {
		t.Fatal(err)
	}
	assertMode(t, path, privateFileMode)
}

// A file written before the modes were tightened is world-readable; replacing
// it must not leave it that way.
func TestWriteNew_OverwriteTightensExistingFile(t *testing.T) {
	skipWithoutUnixModes(t)
	path := filepath.Join(t.TempDir(), "nota-nfse.xml")
	if err := os.WriteFile(path, []byte("antigo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeNew(path, []byte("<NFSe/>"), true); err != nil {
		t.Fatal(err)
	}
	assertMode(t, path, privateFileMode)
}

func TestWriteDPS_CreatesOwnerOnlyDirectory(t *testing.T) {
	skipWithoutUnixModes(t)
	dir := filepath.Join(t.TempDir(), "notas")

	path, err := writeDPS(&config.Config{}, &emitirFlags{outputDir: dir}, "DPS1", "<DPS/>", true)
	if err != nil {
		t.Fatal(err)
	}
	assertMode(t, dir, privateDirMode)
	assertMode(t, path, privateFileMode)
}

// A directory the user already has is theirs: the CLI writes into it without
// changing who can read it.
func TestWriteDPS_LeavesExistingDirectoryAlone(t *testing.T) {
	skipWithoutUnixModes(t)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := writeDPS(&config.Config{}, &emitirFlags{outputDir: dir}, "DPS1", "<DPS/>", true); err != nil {
		t.Fatal(err)
	}
	assertMode(t, dir, 0o755)
}

func TestConfigInit_ForcarTightensExistingFile(t *testing.T) {
	skipWithoutUnixModes(t)
	path := filepath.Join(t.TempDir(), "nfse.yaml")
	if err := os.WriteFile(path, []byte("ambiente: producao\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"config", "init", "--arquivo", path, "--forcar"})
	if err := root.Execute(); err != nil {
		t.Fatalf("config init falhou: %v\n%s", err, out.String())
	}
	assertMode(t, path, privateFileMode)
}

func TestFileStem(t *testing.T) {
	chave := strings.Repeat("7", 50)
	dps := "DPS" + strings.Repeat("1", 42)

	casos := []struct {
		nome       string
		candidatos []string
		esperado   string
	}{
		{"chave valida", []string{chave, dps}, chave},
		{"chave ausente cai para a DPS", []string{"", dps}, dps},
		{"travessia de diretorio", []string{"../../" + chave, dps}, dps},
		{"separador no meio", []string{strings.Repeat("7", 24) + "/" + strings.Repeat("7", 25), dps}, dps},
		{"chave com espaco", []string{" " + chave, dps}, dps},
		{"DPS sem prefixo", []string{strings.Repeat("1", 45)}, ""},
	}
	for _, caso := range casos {
		obtido := fileStem(caso.candidatos...)
		if caso.esperado == "" {
			if !strings.HasPrefix(obtido, "nfse-") || strings.ContainsAny(obtido, `/\.`) {
				t.Errorf("%s: fileStem = %q, esperava um nome por data", caso.nome, obtido)
			}
			continue
		}
		if obtido != caso.esperado {
			t.Errorf("%s: fileStem = %q, esperava %q", caso.nome, obtido, caso.esperado)
		}
	}
}

// The access key in the response names the file, and the response is not
// trusted: a key that tries to climb out of the output directory must not.
func TestEnviar_ResponseCannotChooseWhereTheNFSeIsWritten(t *testing.T) {
	dir := workspace(t)
	if out, err := runEmit(t, dir, "--numero", "21", "--valor", "1500",
		"--descricao", "Consultoria"); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}
	dpsPath := onlyXMLPath(t, dir)

	stubQuery(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idDps":          "../../idDps",
			"chaveAcesso":    "../../fora",
			"nfseXmlGZipB64": gzipBase64(t, `<?xml version="1.0"?><NFSe><infNFSe Id="N1"/></NFSe>`),
			"tipoAmbiente":   2,
		})
	})

	if out, err := execEnviar(t, dir, dpsPath); err != nil {
		t.Fatalf("envio falhou: %v\n%s", err, out)
	}

	esperado := strings.TrimSuffix(dpsPath, "-dps.xml") + "-nfse.xml"
	if _, err := os.Stat(esperado); err != nil {
		t.Errorf("a NFS-e devia estar em %s, com o nome da DPS local: %v", esperado, err)
	}
	for _, fora := range []string{"fora-nfse.xml", "idDps-nfse.xml"} {
		matches, _ := filepath.Glob(filepath.Join(filepath.Dir(dir), "*", fora))
		matches2, _ := filepath.Glob(filepath.Join(filepath.Dir(dir), fora))
		if len(matches)+len(matches2) > 0 {
			t.Errorf("a resposta escolheu onde gravar: %v %v", matches, matches2)
		}
	}
}

func gzipBase64(t *testing.T, content string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := newGzipWriter(&buf)
	if _, err := zw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return encodeBase64(buf.Bytes())
}
