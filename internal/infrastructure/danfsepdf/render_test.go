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
	avisos, err := Render(exemplo(t), Opcoes{}, &saida)
	if err != nil {
		t.Fatalf("Render devolveu erro: %v", err)
	}
	if len(avisos) != 0 {
		t.Errorf("a nota de exemplo cabe na fonte e nao deveria gerar avisos: %q", avisos)
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
	if _, err := Render(nil, Opcoes{}, &bytes.Buffer{}); err == nil {
		t.Fatal("esperava erro ao desenhar um documento inexistente")
	}
}

// The note under the QR Code has to fit in the three lines of item 2.4.3,
// inside a box 4,72 cm wide, and say the whole sentence.
func TestNotaDoQRCode_CabeEmTresLinhas(t *testing.T) {
	p := novaPagina()
	p.pdf.SetFont(fonte, "", corpoMiudo)

	for i, linha := range notaQRCode {
		// The text starts 0,05 cm into the box, and keeps the same distance
		// from the other side.
		if largura := p.pdf.GetStringWidth(p.traduzir(linha)); largura > qrNotaLargura-0.1 {
			t.Errorf("linha %d tem %.2f cm, mais que os %.2f cm do quadro: %q",
				i+1, largura, qrNotaLargura-0.1, linha)
		}
	}

	const frase = "A autenticidade desta NFS-e pode ser verificada pela leitura " +
		"deste código QR ou pela consulta da chave de acesso no portal nacional da NFS-e"
	if junto := strings.Join(notaQRCode[:], " "); junto != frase {
		t.Errorf("a nota mudou:\n esperava %q\n veio     %q", frase, junto)
	}
}

// A description longer than a line wraps inside its box instead of running
// off the page, and the blocks below move down to make room.
func TestRender_DescricaoLongaQuebraEmLinhas(t *testing.T) {
	doc := exemplo(t)
	doc.Servico.Descricao = strings.Repeat("Consultoria em sistemas de informacao ", 20)

	p, err := desenhar(doc, Opcoes{})
	if err != nil {
		t.Fatalf("desenhar devolveu erro: %v", err)
	}
	if p.deslocamento != 0 {
		t.Error("o deslocamento tem de voltar a zero antes do canhoto")
	}

	p = novaPagina()
	p.pdf.SetFont(fonte, "", corpoTexto)
	linhas := p.linhas(doc.Servico.Descricao, larguraCorpo-2*recuo)
	if len(linhas) < 2 {
		t.Fatalf("a descricao deveria ocupar mais de uma linha, ocupou %d", len(linhas))
	}
	for i, linha := range linhas {
		if largura := p.pdf.GetStringWidth(linha); largura > larguraCorpo-2*recuo {
			t.Errorf("linha %d passa da largura do quadro: %.2f cm", i+1, largura)
		}
	}

	pdf := desenhado(t, doc)
	if !strings.Contains(pdf, "Consultoria em sistemas") {
		t.Error("a descricao nao foi para o papel")
	}
	if paginas := strings.Count(pdf, "/Type /Page\n"); paginas != 1 {
		t.Errorf("a descricao longa empurrou o documento para %d paginas", paginas)
	}
}

// A description longer than the page can hold stops with an ellipsis, leaving
// the complementary information its minimum room.
func TestRender_DescricaoGiganteECortada(t *testing.T) {
	p := novaPagina()
	p.pdf.SetFont(fonte, "", corpoTexto)
	largura := larguraCorpo - 2*recuo

	muitas := p.linhas(strings.Repeat("palavra ", 2000), largura)
	cortadas, sobra := cortar(p, muitas, 5, largura)

	if len(cortadas) != 5 || sobra != 0 {
		t.Fatalf("esperava 5 linhas e nada sobrando, vieram %d e %d", len(cortadas), sobra)
	}
	if !strings.HasSuffix(cortadas[4], reticencias) {
		t.Errorf("a ultima linha deveria terminar em reticencias: %q", cortadas[4])
	}
	if l := p.pdf.GetStringWidth(cortadas[4]); l > largura {
		t.Errorf("a linha com reticencias passa da largura: %.2f cm", l)
	}

	poucas, sobra := cortar(p, []string{"uma"}, 5, largura)
	if len(poucas) != 1 || sobra != 4 {
		t.Errorf("uma linha em cinco deveria devolver quatro: %d linhas, sobra %d", len(poucas), sobra)
	}
}

// cp1252 has no Cyrillic, and the translator turns what it lacks into dots,
// silently. The page is still drawn, and the loss is reported.
func TestRender_AvisaCaracteresForaDaFonte(t *testing.T) {
	doc := exemplo(t)
	doc.Tomador.Nome = "Иван Петров"

	avisos, err := Render(doc, Opcoes{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Render devolveu erro: %v", err)
	}
	if len(avisos) != 1 || !strings.Contains(avisos[0], "Иван Петров") {
		t.Errorf("esperava um aviso sobre o nome do tomador, vieram %q", avisos)
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
