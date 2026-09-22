package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edusouza/nfse-emissor-go/internal/domain/danfse"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/danfsepdf"
)

type danfseFlags struct {
	saida        string
	sobrescrever bool

	semCanhoto  bool
	cancelada   bool
	substituida bool
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
para quem emite quando a API do governo foi suspensa, em 03/08/2026.`,
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

	return cmd
}

func runDanfse(cmd *cobra.Command, entrada string, f *danfseFlags) error {
	conteudo, err := os.ReadFile(entrada)
	if err != nil {
		return fmt.Errorf("nao consegui ler %q: %w", entrada, err)
	}

	doc, err := danfse.Parse(conteudo)
	if err != nil {
		return err
	}

	marca, err := marcaDagua(f)
	if err != nil {
		return err
	}
	doc.Marca = marca

	var pdf bytes.Buffer
	if err := danfsepdf.Render(doc, danfsepdf.Opcoes{SemCanhoto: f.semCanhoto}, &pdf); err != nil {
		return err
	}

	destino := f.saida
	if destino == "" {
		destino = trocarExtensaoPorPDF(entrada)
	}
	if err := writeNew(destino, pdf.Bytes(), f.sobrescrever); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "DANFSe gerado\n")
	fmt.Fprintf(out, "  Chave de acesso  %s\n", doc.Identificacao.ChaveAcesso)
	fmt.Fprintf(out, "  Arquivo          %s\n", destino)
	if doc.Marca != danfse.SemMarca {
		fmt.Fprintf(out, "  Marca d'agua     %s\n", doc.Marca)
	}
	if doc.Cabecalho.SemValidadeJuridica {
		fmt.Fprintf(out, "\nA nota e de homologacao: o documento sai com a tarja\n"+
			"\"NFS-e SEM VALIDADE JURIDICA\", como manda a NT 008.\n")
	}
	return nil
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
