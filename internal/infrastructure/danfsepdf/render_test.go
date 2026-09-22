package danfsepdf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/domain/danfse"
)

// desenhado renders the example invoice with compression turned off, so the
// text the page carries can be read back out of the PDF itself. Asserting on
// the drawing rather than on the model is the point: the model was already
// right in the tests of the danfse package, and what can still go wrong here is
// a field that is built and never drawn.
func desenhado(t *testing.T, doc *danfse.Documento) string {
	t.Helper()

	p, err := desenhar(doc)
	if err != nil {
		t.Fatalf("desenhar devolveu erro: %v", err)
	}
	p.pdf.SetCompression(false)

	var saida bytes.Buffer
	if err := p.pdf.Output(&saida); err != nil {
		t.Fatalf("Output devolveu erro: %v", err)
	}
	return saida.String()
}

func exemplo(t *testing.T) *danfse.Documento {
	t.Helper()

	caminho := filepath.Join("..", "..", "domain", "danfse", "testdata", "nfse-exemplo.xml")
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("nao consegui ler a NFS-e de exemplo: %v", err)
	}

	doc, err := danfse.Parse(conteudo)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}
	return doc
}

func TestRender_PaginaA4EmUmaFolhaSo(t *testing.T) {
	var saida bytes.Buffer
	if err := Render(exemplo(t), &saida); err != nil {
		t.Fatalf("Render devolveu erro: %v", err)
	}

	pdf := saida.String()
	if !strings.HasPrefix(pdf, "%PDF-") {
		t.Fatal("a saida nao comeca com o cabecalho de um PDF")
	}

	// 21 x 29,7 cm in points, which is what item 2.2.1 asks for.
	if !strings.Contains(pdf, "/MediaBox [0 0 595.28 841.89]") {
		t.Error("a pagina nao tem o tamanho A4 retrato")
	}

	// Item 2.2 requires a single page; more than one /Type /Page means the
	// content spilled.
	if paginas := strings.Count(pdf, "/Type /Page\n"); paginas != 1 {
		t.Errorf("esperava uma pagina, o PDF tem %d", paginas)
	}
}

func TestRender_CamposDaIdentificacaoVaoParaOPapel(t *testing.T) {
	pdf := desenhado(t, exemplo(t))

	esperados := []string{
		"CHAVE DE ACESSO DA NFS-e",
		"41069022212345678000195000000000000126081234567890",
		"12/08/2026 14:31:05",
		"NFS-e Gerada",
		"Prestador",
		"00001",
	}
	for _, texto := range esperados {
		if !strings.Contains(pdf, texto) {
			t.Errorf("o documento nao traz %q", texto)
		}
	}
}

func TestRender_TarjaSoSaiEmHomologacao(t *testing.T) {
	doc := exemplo(t)
	if !doc.Cabecalho.SemValidadeJuridica {
		t.Fatal("a NFS-e de exemplo deveria ser de homologacao")
	}

	// The banner is drawn in red, and red is what makes it a warning: item
	// 2.4.3 asks for M100/Y100, which is 1 0 0 in the PDF's RGB operator.
	pdf := desenhado(t, doc)
	if !strings.Contains(pdf, "SEM VALIDADE JUR") {
		t.Error("a tarja de homologacao nao foi desenhada")
	}
	if !strings.Contains(pdf, "1.000 0.000 0.000 rg") {
		t.Error("a tarja nao saiu em vermelho")
	}

	doc.Cabecalho.SemValidadeJuridica = false
	if pdf := desenhado(t, doc); strings.Contains(pdf, "SEM VALIDADE JUR") {
		t.Error("uma nota de producao nao pode levar a tarja de homologacao")
	}
}

func TestRender_SemDocumento(t *testing.T) {
	if err := Render(nil, &bytes.Buffer{}); err == nil {
		t.Fatal("esperava erro ao desenhar um documento inexistente")
	}
}

// The note under the QR Code has to fit in the three lines of item 2.4.3,
// inside a box 4,72 cm wide, without dropping a word.
func TestQuebrarEm_NotaDoQRCodeCabeEmTresLinhas(t *testing.T) {
	p := novaPagina()
	p.pdf.SetFont(fonte, "", corpoMiudo)

	linhas := quebrarEm(p.traduzir(notaQRCode), qrNotaLinhas, p.pdf.GetStringWidth)

	if len(linhas) != qrNotaLinhas {
		t.Fatalf("esperava %d linhas, vieram %d", qrNotaLinhas, len(linhas))
	}
	for i, linha := range linhas {
		if largura := p.pdf.GetStringWidth(linha); largura > qrNotaLargura {
			t.Errorf("linha %d tem %.2f cm, mais que os %.2f cm do quadro: %q",
				i+1, largura, qrNotaLargura, linha)
		}
	}

	junto := strings.Join(linhas, " ")
	if junto != p.traduzir(notaQRCode) {
		t.Errorf("a quebra mudou o texto:\n esperava %q\n veio     %q", notaQRCode, junto)
	}
}

func TestQuebrarEm_MenosPalavrasQueLinhas(t *testing.T) {
	largura := func(s string) float64 { return float64(len(s)) }

	linhas := quebrarEm("duas palavras", 3, largura)
	if len(linhas) != 3 {
		t.Fatalf("esperava 3 linhas, vieram %d: %q", len(linhas), linhas)
	}
	if linhas[0] != "duas" || linhas[1] != "palavras" || linhas[2] != "" {
		t.Errorf("quebra inesperada: %q", linhas)
	}
}
