package danfse

import (
	"strings"

	"github.com/beevik/etree"
)

// Prefixes of item 2.1.12, in the order NT 008 lists them. The nota técnica
// gives both the wording and the order, and separates the parts by pipes.
const (
	prefixoInfComp   = "Inf. Cont.:"
	prefixoSubst     = "NFS-e Subst.:"
	prefixoDocRef    = "Doc. Ref.:"
	prefixoObra      = "Cod. Obra:"
	prefixoImovel    = "Insc. Imob.:"
	prefixoEvento    = "Cod. Evt.:"
	prefixoDocTec    = "Doc. Tec.:"
	prefixoPedido    = "Núm. Ped.:"
	prefixoItemPed   = "Item Ped.:"
	prefixoInfATMun  = "Inf. A. T. Mun.:"
	separadorInfComp = " | "
)

// tributosAproximados is the sentence note 10 makes mandatory, whatever else
// the field carries.
const tributosAproximados = "Totais Aproximados dos Tributos cfe. Lei nº 12.741/2012: " +
	"Federais: %F ; Estaduais: %E ; Municipais: %M"

// limiteInfComp is the length NT 008 gives the field, "sem prejuízo à linha de
// Totais Aproximados dos Tributos, que é fixa" — so the tax line is appended
// after the truncation, never cut by it.
const limiteInfComp = 2000

func complementares(inf, infDPS *etree.Element) string {
	infoCompl := caminho(infDPS, "serv/infoCompl")

	partes := []string{
		comPrefixo(prefixoInfComp, texto(infoCompl, "xInfComp")),
		comPrefixo(prefixoSubst, texto(infDPS, "subst/chSubstda")),
		comPrefixo(prefixoDocRef, texto(infoCompl, "docRef")),
		comPrefixo(prefixoObra, texto(infDPS, "serv/obra/cObra")),
		comPrefixo(prefixoImovel, inscricaoImobiliaria(infDPS)),
		comPrefixo(prefixoEvento, texto(infDPS, "serv/atvEvento/idAtvEvt")),
		comPrefixo(prefixoDocTec, texto(infoCompl, "idDocTec")),
		comPrefixo(prefixoPedido, texto(infoCompl, "xPed")),
		comPrefixo(prefixoItemPed, texto(infoCompl, "gItemPed/xItemPed")),
		comPrefixo(prefixoInfATMun, texto(inf, "xOutInf")),
	}

	conteudo := limitar(juntar(separadorInfComp, partes...), limiteInfComp)
	return juntar(separadorInfComp, conteudo, totaisDeTributos(infDPS))
}

// inscricaoImobiliaria looks in both places the leiaute keeps it: the building
// works group of the DPS, and the property group the IBS/CBS layout added.
func inscricaoImobiliaria(infDPS *etree.Element) string {
	if inscricao := texto(infDPS, "serv/obra/inscImobFisc"); inscricao != "" {
		return inscricao
	}
	return texto(infDPS, "IBSCBS/imovel/inscImobFisc")
}

// totaisDeTributos builds the line of the Lei 12.741/2012.
//
// The invoice states the totals as money or as percentages, never both, and it
// may state neither — indTotTrib = 0 is the issuer declaring no estimate. The
// line prints either way, with a dash where there is no number: note 10 makes
// the line mandatory, and leaving it out because the invoice is silent would
// drop a statement the law requires the document to carry.
func totaisDeTributos(infDPS *etree.Element) string {
	totTrib := caminho(infDPS, "valores/trib/totTrib")

	federais, estaduais, municipais := Traco, Traco, Traco

	switch {
	case caminho(totTrib, "vTotTrib") != nil:
		valores := caminho(totTrib, "vTotTrib")
		federais = comMoeda(texto(valores, "vTotTribFed"))
		estaduais = comMoeda(texto(valores, "vTotTribEst"))
		municipais = comMoeda(texto(valores, "vTotTribMun"))

	case caminho(totTrib, "pTotTrib") != nil:
		valores := caminho(totTrib, "pTotTrib")
		federais = campo(percentual(texto(valores, "pTotTribFed")))
		estaduais = campo(percentual(texto(valores, "pTotTribEst")))
		municipais = campo(percentual(texto(valores, "pTotTribMun")))

	case texto(totTrib, "pTotTribSN") != "":
		// The Simples Nacional states one rate for everything it covers, so the
		// three categories cannot be separated. Repeating the same number in
		// all three would invent a split the invoice never made.
		return "Totais Aproximados dos Tributos cfe. Lei nº 12.741/2012: " +
			"Simples Nacional: " + percentual(texto(totTrib, "pTotTribSN"))
	}

	linha := strings.ReplaceAll(tributosAproximados, "%F", federais)
	linha = strings.ReplaceAll(linha, "%E", estaduais)
	return strings.ReplaceAll(linha, "%M", municipais)
}

func comMoeda(valor string) string {
	if valor = strings.TrimSpace(valor); valor == "" {
		return Traco
	}
	return "R$ " + moeda(valor)
}

// comPrefixo labels a value, and disappears when there is no value: an empty
// "Cod. Obra:" on the paper would say the invoice mentions building works.
func comPrefixo(prefixo, valor string) string {
	if valor = strings.TrimSpace(valor); valor == "" {
		return ""
	}
	return prefixo + " " + valor
}
