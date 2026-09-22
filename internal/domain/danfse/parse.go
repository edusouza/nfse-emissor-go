package danfse

import (
	"errors"
	"fmt"
	"strings"

	"github.com/beevik/etree"

	"github.com/edusouza/nfse-emissor-go/internal/domain/query"
)

// Errors returned when the XML is not an NFS-e this package can print.
var (
	// ErrXMLInvalido indicates the bytes are not well-formed XML.
	ErrXMLInvalido = errors.New("o arquivo nao e um XML valido")

	// ErrSemNFSe indicates well-formed XML without an NFS-e inside — most often
	// the DPS that was sent rather than the NFS-e that came back.
	ErrSemNFSe = errors.New("o XML nao contem uma NFS-e")

	// ErrSemChave indicates infNFSe without a usable Id attribute.
	ErrSemChave = errors.New("a NFS-e nao traz a chave de acesso no atributo Id")
)

// consultaPublica is the address the QR Code carries, spelled out in item
// 2.4.3 of NT 008. The access key goes right after the equals sign.
const consultaPublica = "https://www.nfse.gov.br/ConsultaPublica/?tpc=1&chave="

// prefixoID is what TSIdNFSe puts in front of the 50-digit key. NT 008 asks
// for the key without it ("Informar o id da NFS-e sem o prefixo \"NFS\"").
const prefixoID = "NFS"

// Parse reads the XML of an authorised NFS-e and builds the document to print.
//
// It takes the XML the government returned — what `nfse consultar` writes, and
// what `nfse emitir --enviar` saves — never the DPS that was sent: the DANFSe
// represents the invoice that exists, and only the reply carries the access
// key, the number and the status.
func Parse(conteudo []byte) (*Documento, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(conteudo); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrXMLInvalido, err)
	}

	inf := doc.FindElement("//infNFSe")
	if inf == nil {
		return nil, ErrSemNFSe
	}

	chave, err := chaveDeAcesso(inf)
	if err != nil {
		return nil, err
	}

	infDPS := inf.FindElement("DPS/infDPS")

	return &Documento{
		Cabecalho:     cabecalho(inf, infDPS, chave),
		Identificacao: identificacao(inf, infDPS, chave),
	}, nil
}

// chaveDeAcesso extracts the access key from the Id attribute.
//
// The key is checked, not just trimmed: everything else on the document — the
// QR Code above all — is built from it, and a malformed key would print a
// document whose lookup leads nowhere.
func chaveDeAcesso(inf *etree.Element) (string, error) {
	id := strings.TrimSpace(inf.SelectAttrValue("Id", ""))
	if id == "" {
		return "", ErrSemChave
	}

	chave := strings.TrimPrefix(id, prefixoID)
	if err := query.ValidateAccessKey(chave); err != nil {
		return "", fmt.Errorf("%w: %v", ErrSemChave, err)
	}
	return chave, nil
}

func cabecalho(inf, infDPS *etree.Element, chave string) Cabecalho {
	tpAmb := texto(infDPS, "tpAmb")

	return Cabecalho{
		Municipio:           municipioEmitente(inf, infDPS),
		AmbienteGerador:     descrever(ambienteGerador, texto(inf, "ambGer")),
		TipoAmbiente:        descrever(tipoAmbiente, tpAmb),
		SemValidadeJuridica: tpAmb == "2",
		QRCode:              consultaPublica + chave,
	}
}

// municipioEmitente joins the emitter's city and state for the header.
//
// NT 008 asks for the city not to be shown when the national taxation code's
// item is 99 — the item reserved for services outside the list, which has no
// municipality to speak of.
func municipioEmitente(inf, infDPS *etree.Element) string {
	if strings.HasPrefix(texto(infDPS, "serv/cServ/cTribNac"), "99") {
		return ""
	}

	cidade := texto(inf, "xLocEmi")
	uf := texto(inf, "emit/enderNac/UF")
	switch {
	case cidade == "" && uf == "":
		return ""
	case uf == "":
		return cidade
	case cidade == "":
		return uf
	}
	return limitar(cidade+" / "+uf, 37)
}

func identificacao(inf, infDPS *etree.Element, chave string) Identificacao {
	return Identificacao{
		ChaveAcesso: chave,
		Numero:      campo(texto(inf, "nNFSe")),
		Competencia: campo(data(texto(infDPS, "dCompet"))),
		EmissaoNFSe: campo(dataHora(texto(inf, "dhProc"))),
		NumeroDPS:   campo(texto(infDPS, "nDPS")),
		SerieDPS:    campo(texto(infDPS, "serie")),
		EmissaoDPS:  campo(dataHora(texto(infDPS, "dhEmi"))),
		Emitente:    campo(descrever(emitenteDPS, texto(infDPS, "tpEmit"))),
		Situacao:    campo(limitar(descrever(situacaoNFSe, texto(inf, "cStat")), 40)),
		Finalidade:  campo(limitar(descrever(finalidadeNFSe, texto(infDPS, "IBSCBS/finNFSe")), 40)),
	}
}

// texto returns the trimmed content of a child element, or "" when the path
// leads nowhere. A missing branch is ordinary here: whole blocks of the NFS-e
// are optional, and layout v1.00 simply has no IBS/CBS group.
func texto(raiz *etree.Element, caminho string) string {
	if raiz == nil {
		return ""
	}
	elemento := raiz.FindElement(caminho)
	if elemento == nil {
		return ""
	}
	return strings.TrimSpace(elemento.Text())
}
