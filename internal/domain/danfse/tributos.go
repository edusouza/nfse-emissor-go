package danfse

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/beevik/etree"
)

// MensagemSemISSQN is the sentence note 4 of NT 008 puts in place of the whole
// municipal block when the operation is outside the tax.
const MensagemSemISSQN = "TRIBUTAÇÃO MUNICIPAL (ISSQN) - OPERAÇÃO NÃO SUJEITA AO ISSQN"

// tribNaoIncidencia is the code of TSTribISSQN for an operation the municipal
// tax does not reach.
const tribNaoIncidencia = "4"

// pisRetido and cofinsRetido read TSTipoRetPISCofins one contribution at a
// time.
//
// NT 008 writes its rule for code 1 alone — "Quando tpRetPisCofins = 1 (PIS/
// COFINS Retido)" — because that was the whole list when it was drafted. The
// schema now has ten codes, and 3 and 4 withhold both contributions just as 1
// does; 5, 6, 7 and 9 withhold one of them. Applying the rule to code 1 only
// printed a withheld PIS as the issuer's own debt and left it out of the
// withheld total — the double count the rule exists to prevent. So the rule is
// applied as it reads, to each contribution the code says was withheld.
// Whether CSLL was withheld needs no table: vRetCSLL is a withheld amount by
// definition, and is always part of the total.
var (
	pisRetido = map[string]bool{
		"1": true, "3": true, "4": true, "5": true, "9": true,
	}
	cofinsRetido = map[string]bool{
		"1": true, "3": true, "4": true, "6": true, "7": true,
	}
)

func servico(inf, infDPS *etree.Element) Servico {
	cTribNac := texto(infDPS, "serv/cServ/cTribNac")
	cTribMun := texto(infDPS, "serv/cServ/cTribMun")

	// "SE xTribMun <> '' ENTAO Descrição Municipal SENAO Descrição Nacional".
	descricao := texto(inf, "xTribMun")
	if descricao == "" {
		descricao = texto(inf, "xTribNac")
	}

	return Servico{
		CodigoTributacao: campo(juntar(" / ", codigoTributacao(cTribNac), cTribMun)),
		CodigoNBS:        campo(codigoNBS(texto(infDPS, "serv/cServ/cNBS"))),
		LocalPrestacao: campo(localidade(
			texto(inf, "xLocPrestacao"),
			texto(infDPS, "serv/locPrest/cLocPrestacao"),
			texto(infDPS, "serv/locPrest/cPaisPrestacao"))),
		DescricaoCodigo: campo(limitar(descricao, 170)),
		Descricao:       campo(limitar(texto(infDPS, "serv/cServ/xDescServ"), 1300)),
	}
}

// localidade renders "Município / UF / País", as items 2.1.7 and 2.1.8 ask.
//
// The invoice names the municipality but not its state. The state is the
// first two digits of the IBGE code — a fixed table, so no lookup — and a
// national code means the country is Brazil, which the nota técnica writes as
// its ISO code. A place abroad has no IBGE code and carries its country.
func localidade(nome, codigoIBGE, pais string) string {
	if codigoIBGE != "" {
		return limitar(juntar(" / ", nome, siglaUF(codigoIBGE), paisBrasil), 42)
	}
	return limitar(juntar(" / ", nome, pais), 42)
}

// codigoTributacao punctuates the six digits of the national code as the nota
// técnica shows them: nn.nn.nn.
func codigoTributacao(codigo string) string {
	if len(codigo) != 6 || !apenasDigitos(codigo) {
		return codigo
	}
	return codigo[0:2] + "." + codigo[2:4] + "." + codigo[4:6]
}

// codigoNBS punctuates the NBS code as n.nnnn.nn.nn.
func codigoNBS(codigo string) string {
	if len(codigo) != 9 || !apenasDigitos(codigo) {
		return codigo
	}
	return codigo[0:1] + "." + codigo[1:5] + "." + codigo[5:7] + "." + codigo[7:9]
}

func issqn(inf, infDPS *etree.Element) ISSQN {
	tribMun := caminho(infDPS, "valores/trib/tribMun")
	tipo := texto(tribMun, "tribISSQN")

	bloco := ISSQN{
		// Note 4 speaks of operations "às quais não haja a incidência do ISSQN".
		// That is the fourth option of TSTribISSQN and only it: an immune or
		// exported service is still reached by the tax and still has fields of
		// its own in this block — tpImunidade exists for exactly that case.
		Incide: tipo != tribNaoIncidencia,

		TipoTributacao: campo(limitar(descrever(tributacaoISSQN, tipo), 21)),
		MunicipioIncidencia: campo(localidade(
			texto(inf, "xLocIncid"),
			texto(inf, "cLocIncid"),
			texto(tribMun, "cPaisResult"))),

		RegimeEspecial:          campo(limitar(descrever(regimeEspecial, texto(infDPS, "prest/regTrib/regEspTrib")), 27)),
		TipoImunidade:           campo(limitar(descrever(imunidadeISSQN, texto(tribMun, "tpImunidade")), 40)),
		SuspensaoExigibilidade:  campo(limitar(descrever(exigibilidadeSuspensa, texto(tribMun, "exigSusp/tpSusp")), 40)),
		NumeroProcessoSuspensao: campo(limitar(texto(tribMun, "exigSusp/nProcesso"), 30)),

		BeneficioMunicipal:     campo(limitar(descrever(beneficioMunicipal, texto(inf, "valores/tpBM")), 40)),
		CalculoBM:              campo(moeda(texto(inf, "valores/vCalcBM"))),
		TotalDeducoes:          campo(moeda(totalDeducoes(inf, infDPS))),
		DescontoIncondicionado: campo(moeda(texto(infDPS, "valores/vDescCondIncond/vDescIncond"))),

		BaseCalculo: campo(moeda(texto(inf, "valores/vBC"))),
		Aliquota:    campo(percentual(texto(inf, "valores/pAliqAplic"))),
		Retencao:    campo(limitar(descrever(retencaoISSQN, texto(tribMun, "tpRetISSQN")), 25)),
		Apurado:     campo(moeda(texto(inf, "valores/vISSQN"))),
	}

	return bloco
}

// totalDeducoes takes the deduction the NFS-e states, and falls back to the one
// the DPS declared.
//
// The nota técnica points this field at two places at once — vDR in the DPS and
// vCalcDR in the NFS-e. They are the same number seen from both sides: what was
// asked for and what the government computed. The computed one wins when it is
// there, because the document represents the invoice, not the request.
func totalDeducoes(inf, infDPS *etree.Element) string {
	if calculado := texto(inf, "valores/vCalcDR"); calculado != "" {
		return calculado
	}
	return texto(infDPS, "valores/vDedRed/vDR")
}

func federal(infDPS *etree.Element) Federal {
	tribFed := caminho(infDPS, "valores/trib/tribFed")
	piscofins := caminho(tribFed, "piscofins")

	tipo := texto(piscofins, "tpRetPisCofins")
	vPis := texto(piscofins, "vPis")
	vCofins := texto(piscofins, "vCofins")

	// A withheld contribution belongs to the withheld total and stops being a
	// debt of the issuer's own. NT 008 spells out both halves of the move, and
	// printing only one of them would count the same money twice.
	sociais := []string{texto(tribFed, "vRetCSLL")}
	if pisRetido[tipo] {
		sociais = append(sociais, vPis)
		vPis = "0.00"
	}
	if cofinsRetido[tipo] {
		sociais = append(sociais, vCofins)
		vCofins = "0.00"
	}

	return Federal{
		IRRF:                   campo(moeda(texto(tribFed, "vRetIRRF"))),
		ContribuicaoPrevidenc:  campo(moeda(texto(tribFed, "vRetCP"))),
		ContribuicoesSociais:   campo(moeda(somar(sociais...))),
		PIS:                    campo(moeda(vPis)),
		COFINS:                 campo(moeda(vCofins)),
		DescricaoContribuicoes: campo(limitar(descrever(retencaoPisCofins, texto(piscofins, "tpRetPisCofins")), 35)),
	}
}

// ibscbs reads the IBS/CBS block, which only layout v1.01 carries. On a v1.00
// invoice every path below is missing and every field becomes a dash, which is
// note 12 doing its job.
func ibscbs(inf, infDPS *etree.Element) IBSCBS {
	valores := caminho(inf, "IBSCBS/valores")
	totais := caminho(inf, "IBSCBS/totCIBS")
	grupo := caminho(infDPS, "IBSCBS/valores/trib/gIBSCBS")

	return IBSCBS{
		CST: campo(juntar(" / ", texto(grupo, "CST"), texto(grupo, "cClassTrib"))),
		IndicadorOperacao: campo(limitar(juntar(" / ",
			texto(infDPS, "IBSCBS/cIndOp"),
			texto(inf, "IBSCBS/cLocalidadeIncid"),
			texto(inf, "IBSCBS/xLocalidadeIncid")), 56)),

		ExclusoesReducoes: campo(moeda(exclusoesEReducoes(inf, infDPS))),
		BaseCalculo:       campo(moeda(texto(valores, "vBC"))),

		ReducaoAliquota: campo(juntarPercentuais(
			texto(valores, "uf/pRedAliqUF"),
			texto(valores, "mun/pRedAliqMun"),
			texto(valores, "fed/pRedAliqCBS"))),
		AliquotaIBS: campo(juntarPercentuais(
			texto(valores, "uf/pIBSUF"),
			texto(valores, "mun/pIBSMun"))),

		AliquotaEfetivaMun: campo(percentual(texto(valores, "mun/pAliqEfetMun"))),
		ValorApuradoMun:    campo(moeda(texto(totais, "gIBS/gIBSMunTot/vIBSMun"))),
		AliquotaEfetivaUF:  campo(percentual(texto(valores, "uf/pAliqEfetUF"))),
		ValorApuradoUF:     campo(moeda(texto(totais, "gIBS/gIBSUFTot/vIBSUF"))),
		ValorTotalIBS:      campo(moeda(texto(totais, "gIBS/vIBSTot"))),

		AliquotaCBS:        campo(percentual(texto(valores, "fed/pCBS"))),
		AliquotaEfetivaCBS: campo(percentual(texto(valores, "fed/pAliqEfetCBS"))),
		ValorTotalCBS:      campo(moeda(texto(totais, "gCBS/vCBS"))),
	}
}

// exclusoesEReducoes adds up the five values NT 008 lists for this one field.
func exclusoesEReducoes(inf, infDPS *etree.Element) string {
	piscofins := caminho(infDPS, "valores/trib/tribFed/piscofins")

	return somar(
		texto(infDPS, "valores/vDescCondIncond/vDescIncond"),
		texto(inf, "IBSCBS/valores/vCalcReeRepRes"),
		texto(inf, "valores/vISSQN"),
		texto(piscofins, "vPis"),
		texto(piscofins, "vCofins"),
	)
}

func totais(inf, infDPS *etree.Element) Totais {
	ibs := texto(inf, "IBSCBS/totCIBS/gIBS/vIBSTot")
	cbs := texto(inf, "IBSCBS/totCIBS/gCBS/vCBS")

	return Totais{
		ValorServico:           campo(moeda(texto(infDPS, "valores/vServPrest/vServ"))),
		DescontoIncondicionado: campo(moeda(texto(infDPS, "valores/vDescCondIncond/vDescIncond"))),
		DescontoCondicionado:   campo(moeda(texto(infDPS, "valores/vDescCondIncond/vDescCond"))),
		TotalRetencoes:         campo(moeda(texto(inf, "valores/vTotalRet"))),
		ValorLiquido:           campo(moeda(texto(inf, "valores/vLiq"))),
		TotalIBSCBS:            campo(moeda(somar(ibs, cbs))),
		ValorLiquidoComIBSCBS:  campo(moeda(texto(inf, "IBSCBS/totCIBS/vTotNF"))),
	}
}

// juntar joins the parts that have something in them, so a missing piece does
// not leave a dangling separator on the paper.
func juntar(separador string, partes ...string) string {
	presentes := make([]string, 0, len(partes))
	for _, parte := range partes {
		if parte = strings.TrimSpace(parte); parte != "" {
			presentes = append(presentes, parte)
		}
	}
	return strings.Join(presentes, separador)
}

// juntarPercentuais formats each rate and joins them, but only when at least
// one of them exists — a field reading "% / % / %" would be noise.
func juntarPercentuais(valores ...string) string {
	formatados := make([]string, 0, len(valores))
	for _, valor := range valores {
		if valor = strings.TrimSpace(valor); valor != "" {
			formatados = append(formatados, percentual(valor))
		}
	}
	return strings.Join(formatados, " / ")
}

// somar adds decimal values without going through a float, by working in
// hundredths. It returns "" when none of the values exists, so the field falls
// to the dash of note 12 instead of printing a zero the invoice never stated.
func somar(valores ...string) string {
	var centavos int64
	var achou bool

	for _, valor := range valores {
		inteiro, decimal, ok := partesDoNumero(valor)
		if !ok {
			continue
		}
		achou = true

		parcela, ok := paraCentavos(inteiro, decimal)
		if !ok {
			continue
		}
		centavos += parcela
	}

	if !achou {
		return ""
	}
	return deCentavos(centavos)
}

// maxDigitosInteiros is the integer part TSDec15V2 allows: fifteen digits in
// all, two of them after the point. Every monetary value of the NFS-e fits it,
// and so does a sum of a handful of them inside an int64 of hundredths.
const maxDigitosInteiros = 13

// paraCentavos turns the halves partesDoNumero produced into hundredths.
//
// The command does not validate the XML against the schema, so a value can be
// any length. One longer than the schema allows is refused rather than parsed:
// an int64 overflows quietly, and a total that wrapped around would print as a
// plausible — even negative — amount on a fiscal document.
func paraCentavos(inteiro, decimal string) (int64, bool) {
	if len(strings.TrimPrefix(inteiro, "-")) > maxDigitosInteiros {
		return 0, false
	}
	centavos, err := strconv.ParseInt(inteiro+decimal, 10, 64)
	if err != nil {
		return 0, false
	}
	return centavos, true
}

func deCentavos(centavos int64) string {
	sinal := ""
	if centavos < 0 {
		sinal, centavos = "-", -centavos
	}
	return fmt.Sprintf("%s%d.%02d", sinal, centavos/100, centavos%100)
}

// caminho walks to a child element, returning nil when the branch is absent —
// whole groups of the NFS-e are optional, and the IBS/CBS one does not exist at
// all in layout v1.00.
func caminho(raiz *etree.Element, caminhoRelativo string) *etree.Element {
	if raiz == nil {
		return nil
	}
	return raiz.FindElement(caminhoRelativo)
}
