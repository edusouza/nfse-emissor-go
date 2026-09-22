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

// copiarExemplo puts the fictitious NFS-e of the danfse package inside a
// temporary directory, which is also where the PDF will land.
func copiarExemplo(t *testing.T) (dir, xml string) {
	t.Helper()

	origem := filepath.Join("..", "domain", "danfse", "testdata", "nfse-exemplo.xml")
	conteudo, err := os.ReadFile(origem)
	if err != nil {
		t.Fatalf("nao consegui ler a NFS-e de exemplo: %v", err)
	}

	dir = t.TempDir()
	xml = filepath.Join(dir, "nfse-exemplo.xml")
	if err := os.WriteFile(xml, conteudo, 0o644); err != nil {
		t.Fatalf("nao consegui gravar a NFS-e de exemplo: %v", err)
	}
	return dir, xml
}

func rodarDanfse(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := NewRootCommand()
	var saida bytes.Buffer
	cmd.SetOut(&saida)
	cmd.SetErr(&saida)
	cmd.SetArgs(append([]string{"danfse"}, args...))

	err := cmd.Execute()
	return saida.String(), err
}

func TestDanfse_GravaOPDFAoLadoDoXML(t *testing.T) {
	dir, xml := copiarExemplo(t)

	saida, err := rodarDanfse(t, xml)
	if err != nil {
		t.Fatalf("o comando falhou: %v\n%s", err, saida)
	}

	pdf, err := os.ReadFile(filepath.Join(dir, "nfse-exemplo.pdf"))
	if err != nil {
		t.Fatalf("o PDF nao foi gravado: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Error("o arquivo gravado nao e um PDF")
	}

	if !strings.Contains(saida, "41069022212345678000195000000000000126081234567890") {
		t.Errorf("a saida nao mostra a chave de acesso:\n%s", saida)
	}
	// The invoice is a homologation one, and the operator has to know the
	// document carries the banner before taking it to a client.
	if !strings.Contains(saida, "SEM VALIDADE JURIDICA") {
		t.Errorf("a saida nao avisa que a nota e de homologacao:\n%s", saida)
	}
}

func TestDanfse_NaoSobrescreveSemPedir(t *testing.T) {
	dir, xml := copiarExemplo(t)
	destino := filepath.Join(dir, "danfse.pdf")

	if _, err := rodarDanfse(t, xml, "-o", destino); err != nil {
		t.Fatalf("a primeira geracao falhou: %v", err)
	}

	if _, err := rodarDanfse(t, xml, "-o", destino); err == nil {
		t.Fatal("esperava recusa ao gravar por cima de um PDF existente")
	}

	if _, err := rodarDanfse(t, xml, "-o", destino, "--sobrescrever"); err != nil {
		t.Fatalf("com --sobrescrever a geracao deveria passar: %v", err)
	}
}

// The DPS is the document that was sent, not the invoice that exists: it has
// no access key, no number and no status, and a DANFSe built from it would be
// an invention.
func TestDanfse_RecusaUmaDPS(t *testing.T) {
	dir := t.TempDir()
	dps := filepath.Join(dir, "dps.xml")
	if err := os.WriteFile(dps, []byte(`<?xml version="1.0"?><DPS><infDPS Id="DPS1"/></DPS>`), 0o644); err != nil {
		t.Fatalf("nao consegui gravar a DPS: %v", err)
	}

	saida, err := rodarDanfse(t, dps)
	if err == nil {
		t.Fatalf("esperava recusa para uma DPS:\n%s", saida)
	}
	if !strings.Contains(err.Error(), "NFS-e") {
		t.Errorf("a mensagem nao explica o que falta: %v", err)
	}
}

func TestDanfse_ArquivoInexistente(t *testing.T) {
	if _, err := rodarDanfse(t, filepath.Join(t.TempDir(), "nao-existe.xml")); err == nil {
		t.Fatal("esperava erro para um arquivo que nao existe")
	}
}

// Cancelled and replaced are different fates, and the flags cannot both be
// true: the document would carry two contradictory watermarks.
func TestDanfse_MarcasDaguaSeExcluem(t *testing.T) {
	dir, xml := copiarExemplo(t)
	destino := filepath.Join(dir, "danfse.pdf")

	if _, err := rodarDanfse(t, xml, "-o", destino, "--cancelada", "--substituida"); err == nil {
		t.Fatal("esperava recusa ao pedir as duas marcas de uma vez")
	}

	saida, err := rodarDanfse(t, xml, "-o", destino, "--cancelada")
	if err != nil {
		t.Fatalf("o comando falhou: %v\n%s", err, saida)
	}
	if !strings.Contains(saida, "CANCELADA") {
		t.Errorf("a saida nao informa a marca d'agua:\n%s", saida)
	}
}

// The lookup is announced before the codes leave the machine, cached after,
// and never allowed to stop the document.
func TestDanfse_ConsultaDeMunicipios(t *testing.T) {
	var pedidos []string
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos = append(pedidos, r.URL.Path)
		w.Write([]byte(`{"id":3550308,"nome":"São Paulo",
			"microrregiao":{"mesorregiao":{"UF":{"sigla":"SP"}}}}`))
	}))
	defer servidor.Close()

	dir, xml := copiarExemplo(t)
	cache := filepath.Join(dir, "municipios.json")

	saida, err := rodarDanfse(t, xml, "--fonte", servidor.URL, "--cache", cache)
	if err != nil {
		t.Fatalf("o comando falhou: %v\n%s", err, saida)
	}

	if !strings.Contains(saida, "serao consultados em") {
		t.Errorf("a consulta nao foi anunciada antes de acontecer:\n%s", saida)
	}
	if len(pedidos) != 1 {
		t.Fatalf("esperava uma consulta, vieram %d: %v", len(pedidos), pedidos)
	}
	if _, err := os.Stat(cache); err != nil {
		t.Errorf("o cache nao foi gravado: %v", err)
	}

	// The same invoice printed again answers from the cache.
	if _, err := rodarDanfse(t, xml, "--fonte", servidor.URL, "--cache", cache, "--sobrescrever"); err != nil {
		t.Fatalf("a segunda geracao falhou: %v", err)
	}
	if len(pedidos) != 1 {
		t.Errorf("a segunda geracao consultou de novo: %v", pedidos)
	}
}

// A lookup that fails is a line on the report, never a failed command: the
// document still has to reach the person waiting for it.
func TestDanfse_ConsultaQueFalhaNaoImpedeODocumento(t *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer servidor.Close()

	dir, xml := copiarExemplo(t)
	destino := filepath.Join(dir, "danfse.pdf")

	saida, err := rodarDanfse(t, xml, "-o", destino,
		"--fonte", servidor.URL, "--cache", filepath.Join(dir, "cache.json"))
	if err != nil {
		t.Fatalf("a consulta falhou e derrubou o comando: %v\n%s", err, saida)
	}

	if _, err := os.Stat(destino); err != nil {
		t.Fatalf("o PDF nao foi gravado: %v", err)
	}
	if !strings.Contains(saida, "codigo do IBGE no lugar") {
		t.Errorf("a saida nao explica por que o nome nao saiu:\n%s", saida)
	}
}

func TestDanfse_SemRedeNaoConsulta(t *testing.T) {
	var chamou bool
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamou = true
	}))
	defer servidor.Close()

	dir, xml := copiarExemplo(t)

	saida, err := rodarDanfse(t, xml, "--sem-rede",
		"--fonte", servidor.URL, "--cache", filepath.Join(dir, "cache.json"))
	if err != nil {
		t.Fatalf("o comando falhou: %v\n%s", err, saida)
	}

	if chamou {
		t.Error("--sem-rede consultou mesmo assim")
	}
	if strings.Contains(saida, "serao consultados em") {
		t.Errorf("com --sem-rede nao ha consulta a anunciar:\n%s", saida)
	}
}
