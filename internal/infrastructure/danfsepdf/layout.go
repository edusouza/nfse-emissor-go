// Package danfsepdf draws a DANFSe as a one-page A4 PDF.
//
// Every number here comes from the table in item 2.4.5 of NT 008 v1.02, which
// gives each field a height, a width and a position in centimetres relative to
// the margin. The PDF is built in centimetres for that reason: the nota técnica
// says 15,62 and the code says 15.62, with nothing in between to get wrong.
package danfsepdf

// Page geometry, from items 2.2.1 and 2.4.5 of NT 008.
//
// The margin is 0,30 cm because that is where every coordinate in the NT's own
// table starts, and 0,30 + 20,40 + 0,30 is exactly the 21 cm of an A4 sheet.
// It contradicts item 2.2.2, which caps the margin at 0,20 cm — the two parts
// of the nota técnica disagree with each other. The table wins here: it is the
// one that positions the fields, it closes the arithmetic of the page, and the
// model in Annex I is drawn from it.
const (
	larguraPagina = 21.0  // A4 portrait
	alturaPagina  = 29.7  //
	margem        = 0.30  // where every coordinate in the NT's table starts
	larguraCorpo  = 20.40 // the width the table gives to a full-width block
)

// alturaCorpo is what is left of the page once both margins are taken.
const alturaCorpo = alturaPagina - 2*margem

// Line weights, item 2.2.3: half a point for the dividers, one point for the
// page border. fpdf takes them in the document unit, so points become
// centimetres here.
const (
	pontosPorPolegada = 72.0
	cmPorPolegada     = 2.54

	linhaDivisoria = 0.5 / pontosPorPolegada * cmPorPolegada
	linhaBorda     = 1.0 / pontosPorPolegada * cmPorPolegada
)

// Font sizes, item 2.4. The nota técnica names Arial for labels and Microsoft
// Sans Serif for content; both are proprietary and cannot ship inside the
// binary, so the PDF's own Helvetica stands in — it has the same metrics as
// Arial, so text occupies the same space. See ADR 0011.
const (
	fonte       = "Helvetica"
	corpoTitulo = 9.0 // "DANFSe v2.0" and the line below it
	corpoMunic  = 8.0 // the emitter's municipality, top right
	corpoBloco  = 7.0 // block titles, and the labels of the identification block
	corpoTexto  = 7.0 // field content
	corpoMiudo  = 6.0 // field labels, and the note under the QR Code
)

// Grey 5% for the shaded areas of item 2.2.3. 5% density of black is 242 on a
// 0-255 scale (255 * 0.95).
const cinzaClaro = 242

// Header, item 2.4.5.
const (
	cabecalhoAltura = 1.16

	logoX       = 0.49
	logoY       = 0.44
	logoLargura = 4.00
	logoAltura  = 0.85

	tituloX       = 5.41
	tituloLargura = 10.19

	identAmbienteX       = 15.62
	identAmbienteLargura = 5.09
	municipioY           = 0.30
	ambienteGeradorY     = 0.97
	tipoAmbienteY        = 1.22
)

// "DADOS DA NFS-e" block, item 2.1.2.
const (
	dadosY      = 1.48
	dadosAltura = 2.84

	chaveLargura = 15.30
	chaveAltura  = 0.77

	// The three columns the block shares with most others.
	colunaA = 0.30
	colunaB = 5.41
	colunaC = 10.51
	colunaD = 15.62

	colunaLargura = 5.09

	linha1Y     = 2.27
	linha2Y     = 2.96
	linha3Y     = 3.65
	linhaAltura = 0.67

	qrX           = 17.48
	qrY           = 1.67
	qrLado        = 1.52
	qrNotaX       = 15.80
	qrNotaY       = 3.36
	qrNotaLargura = 4.72
	qrNotaAltura  = 0.68

	// Item 2.4.3 fixes the number of lines the note under the code takes.
	qrNotaLinhas = 3
)

// Texts the nota técnica dictates word for word.
const (
	tituloDocumento = "DANFSe v2.0"
	subtituloDoc    = "Documento Auxiliar da NFS-e"
	semValidade     = "NFS-e SEM VALIDADE JURÍDICA"

	notaQRCode = "A autenticidade desta NFS-e pode ser verificada pela leitura " +
		"deste código QR ou pela consulta da chave de acesso no portal nacional da NFS-e"
)

// Blocks from "SERVIÇO PRESTADO" down to "VALOR TOTAL DA NFS-e", item 2.4.5.
//
// The people blocks above them (prestador, tomador, destinatário and
// intermediário) occupy the rows between 4,34 and 12,74.
const (
	blocoAltura = 0.63

	servicoY          = 12.74
	descricaoCodigoY  = 13.39
	descricaoCodigoH  = 0.38
	descricaoServicoY = 13.79

	issqnY      = 14.43
	issqnLinha2 = 15.08
	issqnLinha3 = 15.73
	issqnLinha4 = 16.37

	federalY      = 17.02
	federalLinha2 = 17.67

	ibscbsY      = 18.32
	ibscbsLinha2 = 18.96
	ibscbsLinha3 = 19.61
	ibscbsLinha4 = 20.26

	totaisY      = 20.90
	totaisLinha2 = 21.59
	totaisAltura = 0.67

	// A field that spans two of the four columns.
	larguraDupla = 10.19
)

// Block titles, spelled as item 2.4.1 requires them: bold, seven points and all
// caps.
const (
	tituloServico = "SERVIÇO PRESTADO"
	tituloISSQN   = "TRIBUTAÇÃO MUNICIPAL (ISSQN)"
	tituloFederal = "TRIBUTAÇÃO FEDERAL (EXCETO CBS)"
	tituloIBSCBS  = "TRIBUTAÇÃO IBS / CBS"
	tituloTotais  = "VALOR TOTAL DA NFS-e"
)
