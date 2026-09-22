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

	p, err := desenhar(doc, Opcoes{})
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

	doc, err := danfse.Parse(conteudo, nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}
	return doc
}

func TestRender_PaginaA4EmUmaFolhaSo(t *testing.T) {
	var saida bytes.Buffer
	if err := Render(exemplo(t), Opcoes{}, &saida); err != nil {
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
	if err := Render(nil, Opcoes{}, &bytes.Buffer{}); err == nil {
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

// Items 2.5.1 and 2.5.2: the watermark is what tells a reader the invoice is
// no longer worth anything, so it has to be on the page — and only when asked.
func TestRender_MarcaDagua(t *testing.T) {
	doc := exemplo(t)
	if pdf := desenhado(t, doc); strings.Contains(pdf, "CANCELADA") {
		t.Error("uma nota sem marca nao pode sair carimbada")
	}

	doc.Marca = danfse.MarcaCancelada
	pdf := desenhado(t, doc)
	if !strings.Contains(pdf, "CANCELADA") {
		t.Error("a marca d'agua nao foi desenhada")
	}
	// Grey K35 is 166 on a 0-255 scale. A colour whose three channels match
	// is written with the PDF's grayscale operator, so it reads "0.651 g".
	if !strings.Contains(pdf, "0.651 g") {
		t.Error("a marca d'agua nao saiu no cinza que a NT pede")
	}
	// The diagonal comes from a transformation matrix, not from the text.
	if !strings.Contains(pdf, " cm\n") {
		t.Error("a marca d'agua nao foi rotacionada")
	}
}

func TestRender_CanhotoPodeSerOmitido(t *testing.T) {
	doc := exemplo(t)

	comCanhoto, err := desenhar(doc, Opcoes{})
	if err != nil {
		t.Fatalf("desenhar devolveu erro: %v", err)
	}
	comCanhoto.pdf.SetCompression(false)
	var comSaida bytes.Buffer
	if err := comCanhoto.pdf.Output(&comSaida); err != nil {
		t.Fatalf("Output devolveu erro: %v", err)
	}
	if !strings.Contains(comSaida.String(), "Identifica") {
		t.Error("o canhoto deveria sair por padrao")
	}

	semCanhoto, err := desenhar(doc, Opcoes{SemCanhoto: true})
	if err != nil {
		t.Fatalf("desenhar devolveu erro: %v", err)
	}
	semCanhoto.pdf.SetCompression(false)
	var semSaida bytes.Buffer
	if err := semCanhoto.pdf.Output(&semSaida); err != nil {
		t.Fatalf("Output devolveu erro: %v", err)
	}
	if strings.Contains(semSaida.String(), "Data de Cientifica") {
		t.Error("com --sem-canhoto o bloco nao pode ser desenhado")
	}
}

func TestRender_LinhaDeTributosNoPapel(t *testing.T) {
	pdf := desenhado(t, exemplo(t))

	if !strings.Contains(pdf, "12.741/2012") {
		t.Error("a linha da Lei 12.741/2012 nao chegou ao papel")
	}
}
