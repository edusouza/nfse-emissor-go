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

	var pdf bytes.Buffer
	if err := danfsepdf.Render(doc, &pdf); err != nil {
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
	if doc.Cabecalho.SemValidadeJuridica {
		fmt.Fprintf(out, "\nA nota e de homologacao: o documento sai com a tarja\n"+
			"\"NFS-e SEM VALIDADE JURIDICA\", como manda a NT 008.\n")
	}
	return nil
}

// trocarExtensaoPorPDF turns notas/<chave>-nfse.xml into notas/<chave>-nfse.pdf.
func trocarExtensaoPorPDF(caminho string) string {
	extensao := filepath.Ext(caminho)
	return strings.TrimSuffix(caminho, extensao) + ".pdf"
}
