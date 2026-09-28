package danfse

import (
	"strings"

	"github.com/beevik/etree"

	"github.com/edusouza/nfse-emissor-go/pkg/cnpjcpf"
)

// Sentences that replace a whole block, from notes 2 and 3 of NT 008.
const (
	MensagemSemTomador       = "TOMADOR/ADQUIRENTE DA OPERAÇÃO NÃO IDENTIFICADO NA NFS-e"
	MensagemSemDestinatario  = "DESTINATÁRIO DA OPERAÇÃO NÃO IDENTIFICADO NA NFS-e"
	MensagemSemIntermediario = "INTERMEDIÁRIO DA OPERAÇÃO NÃO IDENTIFICADO NA NFS-e"

	MensagemDestinatarioEhTomador = "O DESTINATÁRIO É O PRÓPRIO TOMADOR/ADQUIRENTE DA OPERAÇÃO"
)

// indDestEhTomador is the code TSRTCIndDest uses for "tomador = adquirente =
// destinatário".
const indDestEhTomador = "0"

// pessoa reads one of the four people blocks.
//
// municipios translates the seven-digit IBGE code into a name, and may answer
// that it does not know — see ADR 0012. The code then prints in place of the
// name, which is worse than the nota técnica asks for and better than a blank
// where the document should say who took the service.
func pessoa(raiz *etree.Element, municipios Municipios) Pessoa {
	if raiz == nil {
		return Pessoa{}
	}

	endereco := caminho(raiz, "end")

	return Pessoa{
		Documento:          campo(documento(raiz)),
		InscricaoMunicipal: campo(limitar(texto(raiz, "IM"), 15)),
		Telefone:           campo(limitar(texto(raiz, "fone"), 20)),
		Nome:               campo(limitar(texto(raiz, "xNome"), 80)),
		Municipio:          campo(municipio(endereco, municipios)),
		CodigoCEP:          campo(codigoECEP(endereco)),
		Endereco:           campo(limitar(logradouro(endereco), 80)),
		Email:              campo(limitar(texto(raiz, "email"), 80)),
	}
}

func prestador(infDPS *etree.Element, municipios Municipios) Prestador {
	raiz := caminho(infDPS, "prest")

	return Prestador{
		Pessoa: pessoa(raiz, municipios),
		SimplesNacional: campo(limitar(
			descrever(simplesNacional, texto(raiz, "regTrib/opSimpNac")), 40)),
		RegimeApuracao: campo(limitar(
			descrever(apuracaoSimplesNacional, texto(raiz, "regTrib/regApTribSN")), 80)),
	}
}

func tomador(infDPS *etree.Element, municipios Municipios) Pessoa {
	raiz := caminho(infDPS, "toma")
	if raiz == nil {
		return Pessoa{Mensagem: MensagemSemTomador}
	}
	return pessoa(raiz, municipios)
}

// destinatario reads the block layout v1.01 added.
//
// The invoice says outright when the recipient is the buyer — indDest = 0 —
// and note 3 has a sentence for exactly that, so the block does not repeat the
// same person twice. An invoice under layout v1.00 has no recipient group at
// all, and the sentence of note 2 is the truthful thing to print: the NFS-e
// really does not identify one.
func destinatario(infDPS *etree.Element, municipios Municipios) Pessoa {
	ibscbs := caminho(infDPS, "IBSCBS")

	if texto(ibscbs, "indDest") == indDestEhTomador {
		return Pessoa{Mensagem: MensagemDestinatarioEhTomador}
	}

	raiz := caminho(ibscbs, "dest")
	if raiz == nil {
		return Pessoa{Mensagem: MensagemSemDestinatario}
	}
	return pessoa(raiz, municipios)
}

func intermediario(infDPS *etree.Element, municipios Municipios) Pessoa {
	raiz := caminho(infDPS, "interm")
	if raiz == nil {
		return Pessoa{Mensagem: MensagemSemIntermediario}
	}
	return pessoa(raiz, municipios)
}

// documento formats whichever identification the person carries.
//
// The leiaute makes these a choice, and each has its own shape on the paper:
// nn.nnn.nnn/nnnn-nn, nnn.nnn.nnn-nn, or a foreign tax number printed as it
// came. cNaoNIF, the code for "there is no NIF", is described rather than
// printed as a bare digit nobody can read.
func documento(raiz *etree.Element) string {
	if cnpj := texto(raiz, "CNPJ"); cnpj != "" {
		return cnpjcpf.FormatCNPJ(cnpj)
	}
	if cpf := texto(raiz, "CPF"); cpf != "" {
		return cnpjcpf.FormatCPF(cpf)
	}
	if nif := texto(raiz, "NIF"); nif != "" {
		return limitar(nif, 40)
	}
	return limitar(descrever(semNIF, texto(raiz, "cNaoNIF")), 40)
}

// semNIF describes TSCodNaoNIF.
var semNIF = map[string]string{
	"0": "Não informado na nota de origem",
	"1": "Dispensado do NIF",
	"2": "Não exigência do NIF",
}

// municipio renders "Nome / UF" for a national address, and the city and state
// the invoice itself carries for a foreign one.
func municipio(endereco *etree.Element, municipios Municipios) string {
	if endereco == nil {
		return ""
	}

	if exterior := caminho(endereco, "endExt"); exterior != nil {
		return limitar(juntar(" / ",
			texto(exterior, "xCidade"),
			texto(exterior, "xEstProvReg"),
			texto(exterior, "cPais")), 37)
	}

	return municipioNacional(texto(endereco, "endNac/cMun"), municipios)
}

// municipioNacional renders "Nome / UF" for a seven-digit IBGE code.
func municipioNacional(codigo string, municipios Municipios) string {
	if codigo == "" {
		return ""
	}

	if municipios != nil {
		if nome, uf, ok := municipios.Nome(codigo); ok {
			return limitar(juntar(" / ", nome, uf), 37)
		}
	}
	// Nobody translated the code. Printing it keeps the field pointing at the
	// right municipality, which a blank would not.
	return codigo
}

// codigoECEP joins the IBGE code and the postcode, as the nota técnica shows
// them: "nnnnnnn / nn.nnn-nnn".
func codigoECEP(endereco *etree.Element) string {
	if endereco == nil {
		return ""
	}

	if exterior := caminho(endereco, "endExt"); exterior != nil {
		return limitar(texto(exterior, "cEndPost"), 21)
	}

	nacional := caminho(endereco, "endNac")
	return limitar(juntar(" / ", texto(nacional, "cMun"), formatarCEP(texto(nacional, "CEP"))), 21)
}

// formatarCEP punctuates eight digits as nn.nnn-nnn.
func formatarCEP(cep string) string {
	cep = strings.TrimSpace(cep)
	if len(cep) != 8 || !apenasDigitos(cep) {
		return cep
	}
	return cep[0:2] + "." + cep[2:5] + "-" + cep[5:8]
}

// logradouro concatenates the parts of the street address, as item 2.4.5 asks:
// "Concatenar todos os campos do endereço. Ex.: ccc, ccc, ccc, ccc".
func logradouro(endereco *etree.Element) string {
	if endereco == nil {
		return ""
	}
	return juntar(", ",
		texto(endereco, "xLgr"),
		texto(endereco, "nro"),
		texto(endereco, "xCpl"),
		texto(endereco, "xBairro"))
}

// Codes of TSEmitenteDPS (infDPS/tpEmit) naming who emitted the DPS.
const (
	emitePrestador     = "1"
	emiteTomador       = "2"
	emiteIntermediario = "3"
)

// completarEmitente fills the emitter's block with what infNFSe/emit says
// about them, wherever the DPS said nothing.
//
// NT 008 maps the provider block to DPS/infDPS/prest, but on a real invoice
// that group is nearly empty: rule E0121 rejects the provider's name in a DPS
// the provider emits, because the government already knows it by the CNPJ.
// The name, the address and the contacts come back in the emit group instead,
// filled from the national register. Rules E1282 and E1285 guarantee emit
// carries the document of the person tpEmit names, so it is the same person —
// described by the one party that knows. Following the mapping to the letter
// printed the provider block as a column of dashes.
//
// The DPS still wins field by field: it is what the nota técnica points at,
// and when it does say something, that is what was declared.
func completarEmitente(doc *Documento, inf, infDPS *etree.Element, municipios Municipios) {
	emit := emitente(inf, municipios)

	switch texto(infDPS, "tpEmit") {
	case emitePrestador:
		doc.Prestador.Pessoa = completar(doc.Prestador.Pessoa, emit)
	case emiteTomador:
		doc.Tomador = completar(doc.Tomador, emit)
	case emiteIntermediario:
		doc.Intermediario = completar(doc.Intermediario, emit)
	}
}

// emitente reads infNFSe/emit, whose address is a flat enderNac rather than
// the end/endNac pair of the DPS.
func emitente(inf *etree.Element, municipios Municipios) Pessoa {
	raiz := caminho(inf, "emit")
	if raiz == nil {
		return Pessoa{}
	}

	endereco := caminho(raiz, "enderNac")

	// Rule E1286 makes the emitter's municipality the one the invoice was
	// issued in, and the invoice names that one itself: no lookup needed.
	var municipio string
	if cidade := texto(inf, "xLocEmi"); cidade != "" {
		municipio = limitar(juntar(" / ", cidade, texto(endereco, "UF")), 37)
	} else {
		municipio = municipioNacional(texto(endereco, "cMun"), municipios)
	}

	return Pessoa{
		Documento:          documento(raiz),
		InscricaoMunicipal: limitar(texto(raiz, "IM"), 15),
		Telefone:           limitar(texto(raiz, "fone"), 20),
		Nome:               limitar(texto(raiz, "xNome"), 80),
		Municipio:          municipio,
		CodigoCEP:          limitar(juntar(" / ", texto(endereco, "cMun"), formatarCEP(texto(endereco, "CEP"))), 21),
		Endereco:           limitar(logradouro(endereco), 80),
		Email:              limitar(texto(raiz, "email"), 80),
	}
}

// completar keeps each field the DPS filled and takes the emitter's for the
// rest. A block replaced by a sentence has nobody in it to complete.
func completar(dps, emit Pessoa) Pessoa {
	if dps.Mensagem != "" {
		return dps
	}

	escolher := func(doDPS, doEmit string) string {
		if doDPS != "" && doDPS != Traco {
			return doDPS
		}
		return campo(doEmit)
	}

	return Pessoa{
		Documento:          escolher(dps.Documento, emit.Documento),
		InscricaoMunicipal: escolher(dps.InscricaoMunicipal, emit.InscricaoMunicipal),
		Telefone:           escolher(dps.Telefone, emit.Telefone),
		Nome:               escolher(dps.Nome, emit.Nome),
		Municipio:          escolher(dps.Municipio, emit.Municipio),
		CodigoCEP:          escolher(dps.CodigoCEP, emit.CodigoCEP),
		Endereco:           escolher(dps.Endereco, emit.Endereco),
		Email:              escolher(dps.Email, emit.Email),
	}
}
