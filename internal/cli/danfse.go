package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edusouza/nfse-emissor-go/internal/domain/danfse"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/danfsepdf"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/ibge"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/xmlsigner"
)

type danfseFlags struct {
	saida        string
	sobrescrever bool

	semCanhoto  bool
	cancelada   bool
	substituida bool

	semRede bool
	fonte   string
	cache   string
}

func newDanfseCommand() *cobra.Command {
	var f danfseFlags

	cmd := &cobra.Command{
		Use:   "danfse <arquivo-da-nfse.xml>",
		Short: "Gera o DANFSe em PDF a partir do XML da NFS-e",
		Long: `Desenha o Documento Auxiliar da NFS-e a partir do XML autorizado.

O XML e o que o 'nfse consultar' grava, ou o que o 'nfse emitir --enviar'
guarda depois da autorizacao — nunca a DPS enviada: o DANFSe representa a nota
que existe, e so a resposta do governo traz a chave de acesso e o numero.

O leiaute segue a Nota Tecnica 008 v1.02, que passou a geracao do documento
para quem emite quando a API do governo foi suspensa, em 03/08/2026.

O XML traz o municipio de cada pessoa como um codigo de 7 digitos, e a NT pede
o nome. O comando consulta o servico publico do IBGE para traduzi-lo e guarda
a resposta em cache, entao a segunda impressao da mesma nota nao depende da
rede. Se a consulta nao responder, o documento sai com o codigo no lugar do
nome — nunca deixa de sair. Use --sem-rede para nao consultar nada.

O comando tambem confere a assinatura do governo no XML e avisa se ela faltar
ou nao conferir com o conteudo. Isso mostra que o XML nao foi alterado depois
de autorizado; nao prova que a nota exista. Essa prova e a consulta pela chave
de acesso no portal nacional, que o QR Code do documento abre.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDanfse(cmd, args[0], &f)
		},
	}

	fl := cmd.Flags()
	fl.StringVarP(&f.saida, "saida", "o", "", "arquivo PDF a gravar (padrao: o mesmo nome do XML)")
	fl.BoolVar(&f.sobrescrever, "sobrescrever", false, "substitui o PDF se ele ja existir")
	fl.BoolVar(&f.semCanhoto, "sem-canhoto", false, "omite o canhoto de recebimento, que a NT deixa opcional")
	fl.BoolVar(&f.cancelada, "cancelada", false, "imprime a marca d'agua CANCELADA")
	fl.BoolVar(&f.substituida, "substituida", false, "imprime a marca d'agua SUBSTITUIDA")
	fl.BoolVar(&f.semRede, "sem-rede", false, "nao consulta o IBGE; usa so o que ja esta em cache")
	fl.StringVar(&f.fonte, "fonte", ibge.DefaultBaseURL, "servidor da consulta de municipios")
	fl.StringVar(&f.cache, "cache", "", "arquivo de cache dos municipios (padrao: o diretorio de cache do sistema)")

	return cmd
}

func runDanfse(cmd *cobra.Command, entrada string, f *danfseFlags) error {
	conteudo, err := lerNFSe(entrada)
	if err != nil {
		return err
	}

	// The watermark is checked before anything is written, so a contradictory
	// pair of flags fails before the lookup goes to the network.
	marca, err := marcaDagua(f)
	if err != nil {
		return err
	}

	destino := f.saida
	if destino == "" {
		destino = trocarExtensaoPorPDF(entrada)
	}
	// And so is the destination: finding out it exists only after the lookups
	// and the drawing sent the codes out for nothing, and a second run with
	// --sobrescrever sent them again. writeNew still refuses at the end, which
	// is what guards against a file that appears in between.
	if !f.sobrescrever {
		if _, err := os.Stat(destino); err == nil {
			return arquivoExistente(destino)
		}
	}

	consulta, cache := consultaDeMunicipios(cmd, f)

	doc, err := danfse.Parse(conteudo, consulta)
	// The answers are kept as soon as they exist. Whatever fails after this —
	// the drawing, the file — the next run should not have to ask again.
	gravarCache(cmd, cache)
	if err != nil {
		return err
	}
	doc.Marca = marca

	var pdf bytes.Buffer
	avisos, err := danfsepdf.Render(doc, danfsepdf.Opcoes{SemCanhoto: f.semCanhoto}, &pdf)
	if err != nil {
		return err
	}

	if err := writeNew(destino, pdf.Bytes(), f.sobrescrever); err != nil {
		return err
	}

	relatarDanfse(cmd, doc, destino, consulta, avisos, xmlsigner.VerifyNFSeSignature(conteudo))
	return nil
}

// maxTamanhoNFSe caps what is read from the XML. An NFS-e is tens of
// kilobytes; a file a hundred times that is not one, and reading it whole
// would only find that out later and at a cost.
const maxTamanhoNFSe = 5 << 20

func lerNFSe(caminho string) ([]byte, error) {
	arquivo, err := os.Open(caminho) // #nosec G304 -- the NFS-e path is the user's own choice
	if err != nil {
		return nil, fmt.Errorf("nao consegui ler %q: %w", caminho, err)
	}
	defer arquivo.Close()

	conteudo, err := io.ReadAll(io.LimitReader(arquivo, maxTamanhoNFSe+1))
	if err != nil {
		return nil, fmt.Errorf("nao consegui ler %q: %w", caminho, err)
	}
	if len(conteudo) > maxTamanhoNFSe {
		return nil, fmt.Errorf("%q tem mais de %d MB, e uma NFS-e tem poucos kilobytes: confira se e o arquivo certo",
			caminho, maxTamanhoNFSe>>20)
	}
	return conteudo, nil
}

// gravarCache writes the lookups out. A failure is worth a line and nothing
// more: the cache holds nothing that cannot be asked for again.
func gravarCache(cmd *cobra.Command, cache *ibge.Cache) {
	if err := cache.Gravar(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "aviso: nao consegui gravar o cache de municipios: %v\n", err)
	}
}

// consultaDeMunicipios assembles the municipality lookup, announcing where the
// codes are about to be sent.
//
// The announcement comes right before the first request, as in `nfse
// onboard`: the codes are public, but sending them tells a third party which
// municipalities this person invoices. When the cache answers everything,
// nothing is sent and nothing is announced.
func consultaDeMunicipios(cmd *cobra.Command, f *danfseFlags) (*ibge.Consulta, *ibge.Cache) {
	cache := ibge.NovoCache(f.cache)

	if f.semRede {
		return ibge.NovaConsulta(cmd.Context(), nil, cache), cache
	}

	client := ibge.New(ibge.Config{BaseURL: f.fonte, UserAgent: AppVersion()})
	consulta := ibge.NovaConsulta(cmd.Context(), client, cache)
	consulta.AntesDeConsultar(func() {
		fmt.Fprintf(cmd.OutOrStdout(),
			"Os codigos de municipio da nota serao consultados em %s.\n"+
				"Use --sem-rede para nao consultar.\n\n",
			client.Host())
	})

	return consulta, cache
}

func relatarDanfse(cmd *cobra.Command, doc *danfse.Documento, destino string,
	consulta *ibge.Consulta, avisos []string, assinatura error) {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "DANFSe gerado\n")
	fmt.Fprintf(out, "  Chave de acesso  %s\n", doc.Identificacao.ChaveAcesso)
	fmt.Fprintf(out, "  Arquivo          %s\n", destino)
	if doc.Marca != danfse.SemMarca {
		fmt.Fprintf(out, "  Marca d'agua     %s\n", doc.Marca)
	}

	// A failed lookup is not an error, but it does change what is on the paper,
	// and whoever hands the document to a client should know why.
	if falhas := consulta.Falhas(); len(falhas) > 0 {
		fmt.Fprintf(out, "\nO nome do municipio nao pode ser consultado, entao o documento\n"+
			"saiu com o codigo do IBGE no lugar:\n")
		for _, falha := range falhas {
			fmt.Fprintf(out, "  %s\n", falha)
		}
	}

	if len(avisos) > 0 {
		fmt.Fprintf(out, "\nO PDF usa as fontes padrao, que nao tem todos os caracteres:\n")
		for _, aviso := range avisos {
			fmt.Fprintf(out, "  %s\n", aviso)
		}
	}

	if aviso := avisoDeAssinatura(assinatura); aviso != "" {
		fmt.Fprintf(out, "\n%s\n", aviso)
	}

	if doc.Cabecalho.SemValidadeJuridica {
		fmt.Fprintf(out, "\nA nota e de homologacao: o documento sai com a tarja\n"+
			"\"NFS-e SEM VALIDADE JURIDICA\", como manda a NT 008.\n")
	}
}

// avisoDeAssinatura explains what the signature check found. The document is
// printed either way — it represents what the XML says — but a DANFSe handed
// to a client vouches for the invoice, and one built from an altered XML
// vouches for something the government never issued.
func avisoDeAssinatura(err error) string {
	const confira = "Confira a nota pela chave de acesso no portal nacional antes de\n" +
		"entregar o documento."

	switch {
	case err == nil:
		return ""
	case errors.Is(err, xmlsigner.ErrNFSeUnsigned):
		return "Aviso: a NFS-e nao traz a assinatura do governo, entao nada garante que\n" +
			"este XML seja o que a Sefin autorizou. " + confira
	case errors.Is(err, xmlsigner.ErrNFSeTampered):
		return "Aviso: a assinatura do governo nao confere com o conteudo do XML: ele foi\n" +
			"alterado depois de autorizado, ou a assinatura nao e do governo. " + confira
	default:
		return fmt.Sprintf("Aviso: nao consegui conferir a assinatura da NFS-e (%v).\n%s", err, confira)
	}
}

// marcaDagua reads the watermark from the flags.
//
// It cannot come from the XML: an NFS-e carries no trace of having been
// cancelled — the cancellation is an event registered apart — and a replaced
// invoice only learns of it through the invoice that replaced it. So the
// person printing has to say, and saying both at once is a contradiction
// rather than two marks.
func marcaDagua(f *danfseFlags) (danfse.Marca, error) {
	switch {
	case f.cancelada && f.substituida:
		return danfse.SemMarca, fmt.Errorf("uma nota e cancelada ou substituida, nunca as duas; escolha uma das marcas")
	case f.cancelada:
		return danfse.MarcaCancelada, nil
	case f.substituida:
		return danfse.MarcaSubstituida, nil
	}
	return danfse.SemMarca, nil
}

// trocarExtensaoPorPDF turns notas/<chave>-nfse.xml into notas/<chave>-nfse.pdf.
func trocarExtensaoPorPDF(caminho string) string {
	extensao := filepath.Ext(caminho)
	return strings.TrimSuffix(caminho, extensao) + ".pdf"
}
