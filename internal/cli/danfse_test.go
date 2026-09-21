package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var pdfDeTeste = []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n%%EOF\n")

func runDanfseCmd(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	t.Setenv(envCertPassword, testCertPassword)

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"danfse", "--config", filepath.Join(dir, "nfse.yaml")}, args...))

	err := root.Execute()
	return out.String(), err
}

// stubADN starts a fake DANFSe service. Unlike the Sefin stub, this needs no
// hook: the command already takes --url, because the ADN address is the part
// of this feature least verified against the real service.
func stubADN(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestDanfse_GravaOPDF(t *testing.T) {
	chave := strings.Repeat("5", 50)

	url := stubADN(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("metodo = %s", r.Method)
		}
		if r.URL.Path != "/"+chave {
			t.Errorf("caminho = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(pdfDeTeste)
	})

	dir := workspace(t)
	out, err := runDanfseCmd(t, dir, chave, "--url", url)
	if err != nil {
		t.Fatalf("danfse falhou: %v\n%s", err, out)
	}

	path := filepath.Join(dir, "notas", chave+"-danfse.pdf")
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("PDF nao foi gravado: %v", err)
	}
	if !bytes.Equal(saved, pdfDeTeste) {
		t.Error("o PDF gravado difere do que o servico devolveu")
	}
	if !strings.Contains(out, "DANFSe salvo") {
		t.Errorf("a saida nao confirma a gravacao:\n%s", out)
	}
}

// Baixar de novo a mesma chave tem de funcionar: a NFS-e e imutavel no
// governo, entao a segunda busca traz o mesmo documento.
func TestDanfse_RepetirSobrescreve(t *testing.T) {
	chave := strings.Repeat("7", 50)
	url := stubADN(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(pdfDeTeste)
	})

	dir := workspace(t)
	if _, err := runDanfseCmd(t, dir, chave, "--url", url); err != nil {
		t.Fatalf("primeira chamada falhou: %v", err)
	}
	if out, err := runDanfseCmd(t, dir, chave, "--url", url); err != nil {
		t.Fatalf("repetir deveria funcionar: %v\n%s", err, out)
	}
}

// O caso que o arquivo nao pode receber: 200 com corpo que nao e PDF.
func TestDanfse_NaoGravaCorpoQueNaoEPDF(t *testing.T) {
	chave := strings.Repeat("6", 50)
	url := stubADN(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>servico em manutencao</body></html>"))
	})

	dir := workspace(t)
	out, err := runDanfseCmd(t, dir, chave, "--url", url)
	if err == nil {
		t.Fatalf("esperava erro\n%s", out)
	}
	if !strings.Contains(err.Error(), "nao e um PDF") {
		t.Errorf("erro = %q", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "notas", chave+"-danfse.pdf")); statErr == nil {
		t.Error("um arquivo foi gravado apesar do corpo nao ser PDF")
	}
}

// 403 e a falha que a ADR prevê: a API DANFSe está documentada no manual dos
// municípios, e o certificado de um prestador pode não ser aceito. A mensagem
// precisa dizer isso, senão o usuário procura o erro no lugar errado.
func TestDanfse_AcessoNegadoExplica(t *testing.T) {
	chave := strings.Repeat("8", 50)
	url := stubADN(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	dir := workspace(t)
	_, err := runDanfseCmd(t, dir, chave, "--url", url)
	if err == nil {
		t.Fatal("esperava erro")
	}
	if !strings.Contains(err.Error(), "manual dos municipios") {
		t.Errorf("o erro nao explica a hipotese mais provavel: %q", err)
	}
}

func TestDanfse_ChaveInvalidaNaoVaiARede(t *testing.T) {
	var chamou bool
	url := stubADN(t, func(w http.ResponseWriter, _ *http.Request) {
		chamou = true
	})

	dir := workspace(t)
	if _, err := runDanfseCmd(t, dir, "123", "--url", url); err == nil {
		t.Error("uma chave curta deveria ser recusada")
	}
	if chamou {
		t.Error("o comando foi a rede com uma chave invalida")
	}
}
