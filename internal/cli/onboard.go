package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/edusouza/nfse-emissor-go/internal/config"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/brasilapi"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/xmlsigner"
	"github.com/edusouza/nfse-emissor-go/pkg/cnpjcpf"
)

// onboardData is what the command managed to discover, together with where
// each value came from. The origin travels with the value because it ends up
// in the generated file's header: months later, "this came from the
// certificate" is the difference between trusting a field and rechecking it.
type onboardData struct {
	cnpj      string
	nome      string
	municipio string
	regime    string
	origens   []string
}

// origem records where a field came from, replacing an earlier answer for the
// same field: the certificate says who the holder is, and the registry may say
// it with the razão social the Receita has on file. The header should show the
// source of the value that ended up in the file, not the history.
func (d *onboardData) origem(campo, fonte string) {
	linha := fmt.Sprintf("%s: %s", campo, fonte)

	for i, anterior := range d.origens {
		if strings.HasPrefix(anterior, campo+": ") {
			d.origens[i] = linha
			return
		}
	}
	d.origens = append(d.origens, linha)
}

func newOnboardCommand() *cobra.Command {
	var (
		certFile string
		password string
		cnpjFlag string
		path     string
		serie    string
		ambiente string
		semRede  bool
		fonte    string
		force    bool
	)

	cmd := &cobra.Command{
		Use:   "onboard",
		Short: "Cria um nfse.yaml ja preenchido a partir do certificado ou do CNPJ",
		Long: `Monta a configuracao do emissor com o minimo de digitacao.

O CNPJ vem do proprio certificado A1 — o ICP-Brasil grava o numero do titular
no certificado — ou de --cnpj. Com ele, o comando consulta o cadastro publico
da Receita Federal (via BrasilAPI) e preenche razao social, codigo IBGE do
municipio e regime tributario.

A consulta envia o seu CNPJ para um servico de terceiros. O comando avisa antes
de sair para a rede e ` + "`--sem-rede`" + ` desliga a consulta: nesse caso o arquivo sai
com o que o certificado informa.

Nada aqui e obrigatorio para emitir: tudo que o comando preenche pode ser
escrito a mao no nfse.yaml.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Checked before the password prompt and before any request: there
			// is no point asking for a secret to produce a file that will be
			// refused.
			if !force {
				if _, err := os.Stat(path); err == nil {
					return fmt.Errorf("%s ja existe; use --forcar para sobrescrever", path)
				} else if !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}

			if certFile == "" && cnpjFlag == "" {
				return errors.New("informe --certificado (o CNPJ sai dele) ou --cnpj")
			}

			if !serieValida(serie) {
				return fmt.Errorf("--serie %q deve ter exatamente 5 digitos", serie)
			}

			if ambiente != config.EnvProducao && ambiente != config.EnvProducaoRestrita {
				return fmt.Errorf("--ambiente %q e invalido (use %q ou %q)",
					ambiente, config.EnvProducaoRestrita, config.EnvProducao)
			}

			out := cmd.OutOrStdout()
			data := &onboardData{}

			if certFile != "" {
				pass, err := resolveCertPassword(password, cmd.Flags().Changed("senha"), cmd.ErrOrStderr())
				if err != nil {
					return err
				}
				if err := fillFromCertificate(out, data, certFile, pass); err != nil {
					return err
				}
			}

			if cnpjFlag != "" {
				clean := cnpjcpf.CleanCNPJ(cnpjFlag)
				if !cnpjcpf.ValidateCNPJ(clean) {
					return fmt.Errorf("--cnpj %q tem digitos verificadores invalidos", cnpjFlag)
				}
				switch {
				case data.cnpj == "":
					data.cnpj = clean
					data.origem("prestador.cnpj", "--cnpj")
				case data.cnpj != clean:
					// Emitting for a CNPJ other than the certificate's is
					// rejected by the Sefin, so this is a mistake worth
					// stopping for rather than silently picking a side.
					return fmt.Errorf("o certificado e do CNPJ %s, mas --cnpj informa %s; "+
						"a Sefin so aceita a DPS assinada pelo certificado do proprio prestador",
						data.cnpj, clean)
				}
			}

			if data.cnpj == "" {
				return fmt.Errorf("nao encontrei um CNPJ no certificado %q; informe --cnpj", certFile)
			}

			if !semRede {
				lookupRegistry(cmd.Context(), out, cmd.ErrOrStderr(), data, fonte)
			}

			rendered, err := config.RenderOnboarded(config.Onboarded{
				Ambiente:           ambiente,
				CertificadoArquivo: certFile,
				CNPJ:               data.cnpj,
				Nome:               data.nome,
				RegimeTributario:   data.regime,
				Municipio:          data.municipio,
				Serie:              serie,
				Origens:            data.origens,
			})
			if err != nil {
				return err
			}

			if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
				return fmt.Errorf("nao foi possivel gravar %q: %w", path, err)
			}

			fmt.Fprintf(out, "\n%s criado.\n", path)
			printPending(out, data, certFile, path)
			return nil
		},
	}

	cmd.Flags().StringVarP(&certFile, "certificado", "c", "", "certificado A1 (.pfx/.p12) de onde ler o CNPJ")
	cmd.Flags().StringVarP(&password, "senha", "s", "", "senha do certificado (prefira "+envCertPassword+")")
	cmd.Flags().StringVar(&cnpjFlag, "cnpj", "", "CNPJ do prestador, se preferir informar direto")
	cmd.Flags().StringVarP(&path, "arquivo", "a", config.DefaultFileName, "caminho do arquivo a criar")
	cmd.Flags().StringVar(&serie, "serie", "00001", "serie da DPS, 5 digitos")
	cmd.Flags().StringVar(&ambiente, "ambiente", config.EnvProducaoRestrita, "producao-restrita | producao")
	cmd.Flags().BoolVar(&semRede, "sem-rede", false, "nao consulta o cadastro publico de CNPJ")
	cmd.Flags().StringVar(&fonte, "fonte", brasilapi.DefaultBaseURL, "URL base da consulta de CNPJ")
	cmd.Flags().BoolVar(&force, "forcar", false, "sobrescreve um arquivo existente")

	return cmd
}

// serieValida checks the DPS series before it reaches the file, where the
// mistake would only surface at 'nfse config check'.
func serieValida(serie string) bool {
	if len(serie) != 5 {
		return false
	}
	for _, r := range serie {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// fillFromCertificate reads what the A1 file already knows about its holder.
func fillFromCertificate(out io.Writer, data *onboardData, file, password string) error {
	raw, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("nao foi possivel ler o certificado: %w", err)
	}

	info, err := xmlsigner.ParsePFX(raw, password)
	if err != nil {
		return fmt.Errorf("nao foi possivel abrir o certificado (senha incorreta ou arquivo invalido): %w", err)
	}

	if cnpj := info.SubjectCNPJ(); cnpj != "" {
		data.cnpj = cnpj
		data.origem("prestador.cnpj", "certificado "+file)
	}
	if nome := info.SubjectHolderName(); nome != "" {
		data.nome = nome
		data.origem("prestador.nome", "certificado "+file)
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Certificado\t%s\n", file)
	fmt.Fprintf(tw, "  Titular\t%s\n", info.GetSubjectCN())
	if cert := info.Certificate; cert != nil {
		fmt.Fprintf(tw, "  Valido ate\t%s\n", cert.NotAfter.Local().Format("02/01/2006"))
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	if cert := info.Certificate; cert != nil && time.Until(cert.NotAfter) < expiryWarningWindow {
		fmt.Fprintf(out, "aviso: o certificado vence em menos de %d dias; rode 'nfse cert info' para os detalhes\n",
			int(expiryWarningWindow.Hours()/24))
	}
	return nil
}

// lookupRegistry fills in what only the public registry knows.
//
// A failure is reported and then dropped: the file is still written with what
// the certificate gave, and the missing fields are listed at the end. Turning a
// third-party outage into a failed onboarding would be trading a minute of
// typing for a dead end.
func lookupRegistry(ctx context.Context, out, errOut io.Writer, data *onboardData, baseURL string) {
	client := brasilapi.New(brasilapi.Config{
		BaseURL:   baseURL,
		UserAgent: AppVersion(),
	})

	fmt.Fprintf(out, "\nConsultando o CNPJ %s no cadastro publico da Receita Federal, via %s...\n",
		cnpjcpf.FormatCNPJ(data.cnpj), client.Host())

	if ctx == nil {
		ctx = context.Background()
	}

	empresa, err := client.ConsultarCNPJ(ctx, data.cnpj)
	if err != nil {
		fmt.Fprintf(errOut, "aviso: a consulta falhou (%v)\n", err)
		fmt.Fprintf(errOut, "aviso: o arquivo sai assim mesmo, com o que ja se sabe; o resto vai listado no fim\n")
		return
	}

	fonte := "consulta em " + client.Host()

	if nome := strings.TrimSpace(empresa.RazaoSocial); nome != "" {
		data.nome = nome
		data.origem("prestador.nome", fonte)
	}

	if codigo := empresa.CodigoMunicipioIBGE.String(); len(codigo) == 7 {
		data.municipio = codigo
		data.origem("prestador.municipio", fonte)
	}

	if regime, ok := config.TaxRegimeFromSimples(empresa.OpcaoPeloMEI, empresa.OpcaoPeloSimples); ok {
		data.regime = regime
		data.origem("prestador.regime_tributario", fonte)
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  Razao social\t%s\n", empresa.RazaoSocial)
	if empresa.Municipio != "" {
		fmt.Fprintf(tw, "  Municipio\t%s/%s (IBGE %s)\n", empresa.Municipio, empresa.UF, empresa.CodigoMunicipioIBGE)
	}
	if data.regime != "" {
		fmt.Fprintf(tw, "  Regime\t%s\n", data.regime)
	}
	if empresa.DescricaoSituacaoCadastral != "" {
		fmt.Fprintf(tw, "  Situacao\t%s\n", empresa.DescricaoSituacaoCadastral)
	}
	_ = tw.Flush()

	if !empresa.Ativa() {
		fmt.Fprintf(errOut, "aviso: a situacao cadastral nao esta ATIVA; a Sefin recusa a emissao nesse estado\n")
	}
	if data.regime == "" {
		fmt.Fprintf(errOut, "aviso: a consulta nao informou a opcao pelo Simples Nacional; "+
			"preencha prestador.regime_tributario a mao\n")
	}
}

// printPending lists what still has to be typed, in the order it will be
// needed, and the command that checks the result.
func printPending(out io.Writer, data *onboardData, certFile, path string) {
	var pending []string

	if certFile == "" {
		pending = append(pending, "certificado.arquivo — caminho do seu .pfx/.p12")
	}
	if data.nome == "" {
		pending = append(pending, "prestador.nome — razao social")
	}
	if data.municipio == "" {
		pending = append(pending, "prestador.municipio — codigo IBGE do municipio, 7 digitos")
	}
	if data.regime == "" {
		pending = append(pending, "prestador.regime_tributario — mei ou me_epp")
	}
	pending = append(pending,
		"padroes.servico.codigo_tributacao_nacional — 6 digitos da lista nacional (LC 116/2003)",
		"padroes.servico.descricao — o que voce presta")

	fmt.Fprintf(out, "\nFalta preencher em %s:\n", path)
	for _, p := range pending {
		fmt.Fprintf(out, "  - %s\n", p)
	}

	fmt.Fprintf(out, "\nDepois:\n  nfse config check\n  nfse emitir --valor 100,00\n")
}
