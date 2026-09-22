package danfsepdf

import (
	"fmt"
	"io"

	"github.com/boombuler/barcode/qr"
	"github.com/go-pdf/fpdf"

	"github.com/edusouza/nfse-emissor-go/internal/domain/danfse"
)

// Render draws the document and writes the PDF to w.
func Render(doc *danfse.Documento, w io.Writer) error {
	p, err := desenhar(doc)
	if err != nil {
		return err
	}
	return p.pdf.Output(w)
}

// desenhar lays the whole document out and stops short of writing it, so that
// tests can turn compression off and read what was actually drawn.
func desenhar(doc *danfse.Documento) (*pagina, error) {
	if doc == nil {
		return nil, fmt.Errorf("nao ha DANFSe para desenhar")
	}

	p := novaPagina()
	p.cabecalho(doc.Cabecalho)
	p.dadosDaNFSe(doc.Identificacao)
	p.servico(doc.Servico)
	p.issqn(doc.ISSQN)
	p.federal(doc.Federal)
	p.ibscbs(doc.IBSCBS)
	p.totais(doc.Totais)
	if err := p.qrCode(doc.Cabecalho.QRCode); err != nil {
		return nil, err
	}
	return p, nil
}

// pagina carries the fpdf document and the translator the core fonts need.
type pagina struct {
	pdf *fpdf.Fpdf

	// traduzir converts UTF-8 into the cp1252 encoding the PDF's built-in fonts
	// use. Without it every accented letter of a Brazilian invoice — the ç of
	// "Serviço", the ã of "Informações" — comes out as mojibake.
	traduzir func(string) string
}

func novaPagina() *pagina {
	pdf := fpdf.NewCustom(&fpdf.InitType{
		UnitStr: "cm",
		Size:    fpdf.SizeType{Wd: larguraPagina, Ht: alturaPagina},
	})
	pdf.SetMargins(margem, margem, margem)
	// The DANFSe is one page by rule (item 2.2): an automatic page break would
	// silently produce a document the nota técnica does not allow.
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()

	p := &pagina{pdf: pdf, traduzir: pdf.UnicodeTranslatorFromDescriptor("cp1252")}
	p.pdf.SetDrawColor(0, 0, 0)
	p.pdf.SetLineWidth(linhaBorda)
	p.pdf.Rect(margem, margem, larguraCorpo, alturaCorpo, "D")
	p.pdf.SetLineWidth(linhaDivisoria)
	return p
}

func (p *pagina) cabecalho(c danfse.Cabecalho) {
	p.quadroSombreado(margem, margem, larguraCorpo, cabecalhoAltura)

	p.caixa(logoX, logoY, logoLargura, logoAltura)
	// The nota técnica puts the official NFS-e logo here, and it is published
	// at gov.br rather than inside the schema package. Until the image is at
	// hand the box carries the name it stands for, which keeps the header
	// honest about what it is instead of leaving a blank rectangle.
	p.centralizar("NFS-e", logoX, logoX+logoLargura, logoY+0.58, fonte, "B", corpoTitulo)

	meio := tituloX + tituloLargura/2
	p.centralizarEm(tituloDocumento, meio, margem+0.38, fonte, "B", corpoTitulo)
	p.centralizarEm(subtituloDoc, meio, margem+0.73, fonte, "B", corpoTitulo)
	if c.SemValidadeJuridica {
		// Item 2.4.3: red, bold, nine points, under the subtitle. A test invoice
		// that can pass for a real one is the whole reason this line exists.
		p.pdf.SetTextColor(255, 0, 0)
		p.centralizarEm(semValidade, meio, margem+1.08, fonte, "B", corpoTitulo)
		p.pdf.SetTextColor(0, 0, 0)
	}

	if c.Municipio != "" {
		p.escrever("Município: "+c.Municipio, identAmbienteX+0.08, municipioY+0.42, fonte, "", corpoMunic)
	}
	p.escrever(c.AmbienteGerador, identAmbienteX+0.08, ambienteGeradorY+0.18, fonte, "", corpoMiudo)
	p.escrever(c.TipoAmbiente, identAmbienteX+0.08, tipoAmbienteY+0.18, fonte, "", corpoMiudo)
}

func (p *pagina) dadosDaNFSe(id danfse.Identificacao) {
	// Item 2.4.2: the labels of this block, and only of this block, are seven
	// points and all caps.
	campo := func(rotulo, valor string, x, y, largura, altura float64) {
		p.caixa(x, y, largura, altura)
		p.escrever(rotulo, x+0.08, y+0.28, fonte, "B", corpoBloco)
		p.escrever(valor, x+0.08, y+0.60, fonte, "", corpoTexto)
	}

	campo("CHAVE DE ACESSO DA NFS-e", id.ChaveAcesso, colunaA, dadosY, chaveLargura, chaveAltura)

	campo("NÚMERO DA NFS-e", id.Numero, colunaA, linha1Y, colunaLargura, linhaAltura)
	campo("COMPETÊNCIA DA NFS-e", id.Competencia, colunaB, linha1Y, colunaLargura, linhaAltura)
	campo("DATA E HORA DA EMISSÃO DA NFS-e", id.EmissaoNFSe, colunaC, linha1Y, colunaLargura, linhaAltura)

	campo("NÚMERO DA DPS", id.NumeroDPS, colunaA, linha2Y, colunaLargura, linhaAltura)
	campo("SÉRIE DA DPS", id.SerieDPS, colunaB, linha2Y, colunaLargura, linhaAltura)
	campo("DATA E HORA DA EMISSÃO DA DPS", id.EmissaoDPS, colunaC, linha2Y, colunaLargura, linhaAltura)

	// Item 2.2.3 shades this one field along with the block titles.
	p.quadroSombreado(colunaA, linha3Y, colunaLargura, linhaAltura)
	campo("EMITENTE DA NFS-e", id.Emitente, colunaA, linha3Y, colunaLargura, linhaAltura)
	campo("SITUAÇÃO DA NFS-e", id.Situacao, colunaB, linha3Y, colunaLargura, linhaAltura)
	campo("FINALIDADE", id.Finalidade, colunaC, linha3Y, colunaLargura, linhaAltura)
}

// qrCode draws the code as vector squares rather than as an embedded image.
//
// The library is used only as an encoder: it answers which modules are dark,
// and each dark module becomes a filled rectangle. That keeps the code sharp at
// any printer resolution and spares the document an image object.
func (p *pagina) qrCode(endereco string) error {
	codigo, err := qr.Encode(endereco, qr.M, qr.Auto)
	if err != nil {
		return fmt.Errorf("nao consegui gerar o QR Code da consulta publica: %w", err)
	}

	modulos := codigo.Bounds().Dx()
	lado := qrLado / float64(modulos)

	p.pdf.SetFillColor(0, 0, 0)
	for y := 0; y < modulos; y++ {
		for x := 0; x < modulos; x++ {
			vermelho, verde, azul, _ := codigo.At(x, y).RGBA()
			if vermelho == 0 && verde == 0 && azul == 0 {
				p.pdf.Rect(qrX+float64(x)*lado, qrY+float64(y)*lado, lado, lado, "F")
			}
		}
	}

	p.notaDoQRCode()
	return nil
}

// notaDoQRCode writes the sentence item 2.4.3 requires under the code, in the
// three lines it asks for.
func (p *pagina) notaDoQRCode() {
	p.pdf.SetFont(fonte, "", corpoMiudo)

	linhas := quebrarEm(p.traduzir(notaQRCode), qrNotaLinhas, p.pdf.GetStringWidth)
	altura := qrNotaAltura / float64(qrNotaLinhas)
	for i, linha := range linhas {
		p.pdf.Text(qrNotaX+0.05, qrNotaY+altura*float64(i+1)-0.04, linha)
	}
}

// quadroSombreado fills an area with the 5% grey of item 2.2.3.
func (p *pagina) quadroSombreado(x, y, largura, altura float64) {
	p.pdf.SetFillColor(cinzaClaro, cinzaClaro, cinzaClaro)
	p.pdf.Rect(x, y, largura, altura, "F")
}

// caixa draws a field's dividing lines.
func (p *pagina) caixa(x, y, largura, altura float64) {
	p.pdf.Rect(x, y, largura, altura, "D")
}

func (p *pagina) escrever(texto string, x, y float64, familia, estilo string, tamanho float64) {
	if texto == "" {
		return
	}
	p.pdf.SetFont(familia, estilo, tamanho)
	p.pdf.Text(x, y, p.traduzir(texto))
}

// centralizar puts text in the middle of the span between esquerda and direita.
func (p *pagina) centralizar(texto string, esquerda, direita, y float64, familia, estilo string, tamanho float64) {
	p.centralizarEm(texto, (esquerda+direita)/2, y, familia, estilo, tamanho)
}

func (p *pagina) centralizarEm(texto string, meio, y float64, familia, estilo string, tamanho float64) {
	if texto == "" {
		return
	}
	p.pdf.SetFont(familia, estilo, tamanho)
	traduzido := p.traduzir(texto)
	p.pdf.Text(meio-p.pdf.GetStringWidth(traduzido)/2, y, traduzido)
}
