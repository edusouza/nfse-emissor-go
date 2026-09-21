package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/edusouza/nfse-emissor-go/internal/config"
	"github.com/edusouza/nfse-emissor-go/internal/domain/query"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/adn"
)

type danfseFlags struct {
	configPath   string
	certPath     string
	password     string
	outputDir    string
	url          string
	sobrescrever bool
}

func newDanfseCommand() *cobra.Command {
	var f danfseFlags

	cmd := &cobra.Command{
		Use:   "danfse <chave-de-acesso>",
		Short: "Baixa o PDF (DANFSe) de uma NFS-e emitida",
		Long: `Baixa o Documento Auxiliar da NFS-e — o PDF que se entrega ao cliente.

O emissor nao desenha o documento: quem o gera e o governo, a partir do XML
que ja tem. Por isso o comando e um download, e o binario nao carrega
biblioteca de PDF nenhuma.

O servico fica no Ambiente de Dados Nacional (ADN), e nao na Sefin Nacional —
la o endereco antigo responde 501. Use --url para apontar para outro servidor.

ATENCAO: a API DANFSe e descrita no manual dos municipios; o manual dos
contribuintes nao a menciona, e o swagger de contribuinte nao a traz. Se o
seu certificado de prestador nao for aceito, o comando diz isso em vez de
gravar um arquivo quebrado.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDanfse(cmd, args[0], &f)
		},
	}

	fl := cmd.Flags()
	fl.StringVarP(&f.configPath, "config", "c", config.DefaultFileName, "arquivo de configuracao")
	fl.StringVar(&f.certPath, "cert", "", "certificado PFX/P12 (sobrescreve o da configuracao)")
	fl.StringVarP(&f.password, "senha", "s", "", "senha do certificado (prefira "+envCertPassword+")")
	fl.StringVarP(&f.outputDir, "saida", "o", "", "diretorio onde gravar o PDF")
	fl.StringVar(&f.url, "url", "", "URL base do servico DANFSe no ADN")
	fl.BoolVar(&f.sobrescrever, "sobrescrever", false, "substitui um arquivo ja existente")

	return cmd
}

func runDanfse(cmd *cobra.Command, chave string, f *danfseFlags) error {
	// Catch a malformed key before spending a round-trip and a password prompt.
	if err := query.ValidateAccessKey(chave); err != nil {
		return err
	}

	cfg, err := config.Load(f.configPath)
	if err != nil {
		return err
	}
	if err := cfg.ValidateForQuery(); err != nil {
		return err
	}

	certInfo, err := loadCertificate(cmd, cfg, &emitirFlags{
		configPath: f.configPath,
		certPath:   f.certPath,
		password:   f.password,
	})
	if err != nil {
		return err
	}
	tlsCert, err := certInfo.TLSCertificate()
	if err != nil {
		return fmt.Errorf("certificado nao pode ser usado na conexao: %w", err)
	}

	client := adn.New(adn.Config{
		BaseURL:     f.url,
		Environment: cfg.EnvironmentCode(),
		Certificate: tlsCert,
		UserAgent:   AppVersion(),
	})

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Baixando o DANFSe em %s...\n", client.BaseURL())

	pdf, err := client.BaixarDANFSe(cmd.Context(), chave)
	if err != nil {
		return err
	}

	path, err := gravarDANFSe(cfg, f, chave, pdf)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\nDANFSe salvo\n")
	fmt.Fprintf(out, "  Chave de acesso  %s\n", chave)
	fmt.Fprintf(out, "  Arquivo          %s (%s)\n", path, tamanhoLegivel(len(pdf)))
	return nil
}

func gravarDANFSe(cfg *config.Config, f *danfseFlags, chave string, pdf []byte) (string, error) {
	dir := f.outputDir
	if dir == "" {
		dir = cfg.Saida.Diretorio
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("nao foi possivel criar o diretorio de saida: %w", err)
	}

	// Like a query, a download is idempotent: the NFS-e is immutable at the
	// government, so fetching the same key twice brings the same document.
	// Refusing to overwrite would turn a harmless repeat into an error.
	path := filepath.Join(dir, chave+"-danfse.pdf")
	if err := writeNew(path, pdf, true); err != nil {
		return "", err
	}
	return path, nil
}

// tamanhoLegivel renders a byte count the way a person reads it.
func tamanhoLegivel(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}
