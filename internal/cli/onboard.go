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
	"github.com/edusouza/nfse-emissor-go/internal/domain/municipio"
	"github.com/edusouza/nfse-emissor-go/internal/domain/servico"
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

	// servico is cTribNac, and it is the one field nothing can discover:
	// it says what the provider does, and only the provider knows that. It is
	// filled from --servico or left blank.
	servico servico.Servico

	// cnae and sugestoes carry the registry's activity code and the codes it
	// resembles, which go into the file commented out for the user to pick
	// from. See config.Onboarded.SugestoesServico for why they stay comments.
	cnae      string
	cnaeDesc  string
	sugestoes []servico.Resultado
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
		certFile      string
		password      string
		cnpjFlag      string
		servicoFlag   string
		municipioFlag string
		path          string
		serie         string
		ambiente      string
		semRede       bool
		fonte         string
		force         bool
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

O codigo de tributacao nacional do servico (cTribNac) e o unico campo que
nenhuma consulta responde: ele depende do que voce presta, nao de quem voce e.
Informe em --servico se ja souber; senao, o comando usa o CNAE do cadastro para
sugerir candidatos, e deixa a escolha para voce. Para procurar:

  nfse servico buscar "o que voce faz"

Com --sem-rede, --municipio resolve o codigo IBGE localmente a partir do nome:
a tabela do IBGE esta embutida no binario.

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

			if municipioFlag != "" {
				m, err := municipio.Resolver(municipioFlag)
				if err != nil {
					return erroDeMunicipio(err)
				}
				data.municipio = m.Codigo
				data.origem("prestador.municipio", "--municipio")
				fmt.Fprintf(out, "Municipio    %s (IBGE %s)\n", m, m.Codigo)
			}

			if servicoFlag != "" {
				s, ok := servico.PorCodigo(servicoFlag)
				if !ok {
					return fmt.Errorf("--servico %q nao esta na lista nacional (%s);\n"+
						"procure o codigo com 'nfse servico buscar <termo>'", servicoFlag, servico.Anexo)
				}
				data.servico = s
				data.origem("padroes.servico.codigo_tributacao_nacional", "--servico")
			}

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

			sugerirServico(out, data)

			rendered, err := config.RenderOnboarded(config.Onboarded{
				Ambiente:           ambiente,
				CertificadoArquivo: certFile,
				CNPJ:               data.cnpj,
				Nome:               data.nome,
				RegimeTributario:   data.regime,
				Municipio:          data.municipio,
				Serie:              serie,
				Servico:            data.servico.Codigo,
				ServicoDescricao:   umaLinha(data.servico.Descricao, comentarioLargura),
				SugestoesServico:   sugestoesParaOArquivo(data.sugestoes),
				CNAE:               formatCNAE(data.cnae),
				CNAEDescricao:      umaLinha(data.cnaeDesc, comentarioLargura),
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
	cmd.Flags().StringVar(&servicoFlag, "servico", "", "codigo de tributacao nacional (cTribNac), 6 digitos")
	cmd.Flags().StringVar(&municipioFlag, "municipio", "", `municipio do prestador: codigo IBGE ou "Cidade/UF"`)
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
		switch {
		case data.municipio == "":
			data.municipio = codigo
			data.origem("prestador.municipio", fonte)
		case data.municipio != codigo:
			avisarDivergenciaDeMunicipio(errOut, data.municipio, codigo)
		}
	}

	if regime, ok := config.TaxRegimeFromSimples(empresa.OpcaoPeloMEI, empresa.OpcaoPeloSimples); ok {
		data.regime = regime
		data.origem("prestador.regime_tributario", fonte)
	}

	// The CNAE does not go into the file — it is not a field of the DPS. It is
	// kept because it is the only thing the registry knows about what the
	// provider does, and the service code has to come from somewhere.
	data.cnae = empresa.CNAEFiscal.String()
	data.cnaeDesc = strings.TrimSpace(empresa.CNAEFiscalDescricao)

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  Razao social\t%s\n", empresa.RazaoSocial)
	if empresa.Municipio != "" {
		fmt.Fprintf(tw, "  Municipio\t%s/%s (IBGE %s)\n", empresa.Municipio, empresa.UF, empresa.CodigoMunicipioIBGE)
	}
	if data.regime != "" {
		fmt.Fprintf(tw, "  Regime\t%s\n", data.regime)
	}
	if data.cnae != "" {
		fmt.Fprintf(tw, "  Atividade\t%s %s\n", formatCNAE(data.cnae), data.cnaeDesc)
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

// comentarioLargura is how much of a description fits on one comment line of
// the generated file without wrapping in an editor.
const comentarioLargura = 64

// sugestoesMax is how many candidates are offered. Three is a list someone
// reads; ten is a list someone scrolls past.
const sugestoesMax = 3

// sugerirServico offers candidate cTribNac codes from the provider's CNAE.
//
// It never picks one. The CNAE classifies the company's economic activity for
// the Receita Federal and cTribNac classifies the service for the ISS: there
// is no official correspondence between the two, and no public service
// converts one into the other. What is possible is to rank the national list
// against the words of the CNAE description and let the person recognize their
// own trade — which is also why the CNAE that produced the list is printed
// with it.
func sugerirServico(out io.Writer, data *onboardData) {
	if data.servico.Codigo != "" {
		fmt.Fprintf(out, "\nServico %s — %s\n", data.servico.Codigo,
			umaLinha(data.servico.Descricao, larguraTexto-12))
		return
	}

	if data.cnaeDesc == "" {
		return
	}

	data.sugestoes = servico.SugerirPorCNAE(data.cnaeDesc, sugestoesMax)
	if len(data.sugestoes) == 0 {
		return
	}

	fmt.Fprintf(out, "\nCodigos de servico parecidos com a sua atividade (%s):\n", formatCNAE(data.cnae))
	for _, r := range data.sugestoes {
		fmt.Fprintf(out, "  %s  %s\n", r.Servico.Codigo, umaLinha(r.Servico.Descricao, larguraTexto-10))
	}
	fmt.Fprintf(out, "Sao palpites a partir do texto do CNAE, nao um mapeamento oficial.\n")
	fmt.Fprintf(out, "Confira com 'nfse servico ver <codigo>' ou procure com 'nfse servico buscar'.\n")
}

// sugestoesParaOArquivo shortens the candidates to one comment line each.
func sugestoesParaOArquivo(resultados []servico.Resultado) []config.SugestaoServico {
	var out []config.SugestaoServico
	for _, r := range resultados {
		out = append(out, config.SugestaoServico{
			Codigo:    r.Servico.Codigo,
			Descricao: umaLinha(r.Servico.Descricao, comentarioLargura),
		})
	}
	return out
}

// formatCNAE renders the activity code the way the Receita writes it:
// 6209-1/00.
func formatCNAE(cnae string) string {
	if len(cnae) != 7 {
		return cnae
	}
	return cnae[:4] + "-" + cnae[4:5] + "/" + cnae[5:]
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
		pending = append(pending, "prestador.municipio — codigo IBGE do municipio, 7 digitos; "+
			"ache o seu com 'nfse municipio buscar <nome>', ou reexecute com --municipio \"Cidade/UF\"")
	}
	if data.regime == "" {
		pending = append(pending, "prestador.regime_tributario — mei ou me_epp")
	}
	if data.servico.Codigo == "" {
		linha := "padroes.servico.codigo_tributacao_nacional — 6 digitos da lista nacional (LC 116/2003)"
		if len(data.sugestoes) > 0 {
			linha += fmt.Sprintf("; os candidatos acima estao no arquivo, comentados (o mais proximo e %s)",
				data.sugestoes[0].Servico.Codigo)
		} else {
			linha += "; procure com 'nfse servico buscar <termo>'"
		}
		pending = append(pending, linha)
	}
	pending = append(pending, "padroes.servico.descricao — o que voce presta")

	fmt.Fprintf(out, "\nFalta preencher em %s:\n", path)
	for _, p := range pending {
		for i, linha := range quebrar(p, larguraTexto-4) {
			prefixo := "  - "
			if i > 0 {
				prefixo = "    "
			}
			fmt.Fprintf(out, "%s%s\n", prefixo, linha)
		}
	}

	fmt.Fprintf(out, "\nDepois:\n  nfse config check\n  nfse emitir --valor 100,00\n")
}
