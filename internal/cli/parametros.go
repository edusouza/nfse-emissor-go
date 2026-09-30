package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/edusouza/nfse-emissor-go/internal/config"
	"github.com/edusouza/nfse-emissor-go/internal/domain/servico"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/ibge"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/parametrizacao"
)

type parametrosFlags struct {
	configPath  string
	certPath    string
	password    string
	competencia string
	complemento string
	semCache    bool
}

// newParametrizacaoClient is a seam: tests replace it to reach a stub server
// instead of the ADN.
var newParametrizacaoClient = parametrizacao.New

func newParametrosCommand() *cobra.Command {
	var f parametrosFlags

	cmd := &cobra.Command{
		Use:   "parametros <municipio> <codigo-servico>",
		Short: "Consulta o convenio e a aliquota de ISS que o municipio parametrizou",
		Long: `Consulta no ADN os parametros municipais de um servico: se o convenio do
municipio com o Sistema Nacional esta ativo e qual aliquota do ISSQN ele
parametrizou.

O municipio e o codigo IBGE de 7 digitos ou o nome com a UF ("Curitiba/PR").
O codigo do servico e o cTribNac (010701 ou 01.07.01); o complemento municipal
e 000, a menos que o municipio tenha criado um codigo proprio abaixo do
servico: nesse caso, informe-o com --complemento.

Com --competencia, mostra a aliquota vigente naquela data. Sem ela, mostra o
historico inteiro.

A consulta usa o certificado A1 da configuracao, em TLS mutuo, e o ambiente
dela: em producao-restrita os dados sao de teste. A aliquota que vale e a de
producao.

As respostas ficam guardadas por 24 horas no diretorio de cache do sistema; a
saida diz quando uma veio de la. --sem-cache consulta o ADN de novo.`,
		Example: `  nfse parametros 4106902 010701 --competencia 2026-09-01
  nfse parametros Curitiba/PR 01.07.01
  nfse parametros 4106902 01.07.01 --complemento 001`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runParametros(cmd, args, &f)
		},
	}

	fl := cmd.Flags()
	fl.StringVarP(&f.configPath, "config", "c", config.DefaultFileName, "arquivo de configuracao")
	fl.StringVar(&f.certPath, "cert", "", "certificado PFX/P12 (sobrescreve o da configuracao)")
	fl.StringVarP(&f.password, "senha", "s", "", "senha do certificado (prefira "+envCertPassword+")")
	fl.StringVar(&f.competencia, "competencia", "", "data de competencia (AAAA-MM-DD); sem ela, mostra o historico")
	fl.StringVar(&f.complemento, "complemento", "", "complemento municipal do servico, 3 digitos (padrao: "+servico.ComplementoPadrao+")")
	fl.BoolVar(&f.semCache, "sem-cache", false, "consulta o ADN mesmo que haja resposta guardada das ultimas 24 horas")

	return cmd
}

func runParametros(cmd *cobra.Command, args []string, f *parametrosFlags) error {
	// Everything that can be checked offline is, before the password prompt
	// and before the network: a typo should not cost a round trip.
	codigo, err := servico.CodigoCompleto(args[1], f.complemento)
	if err != nil {
		return err
	}

	var competencia time.Time
	if f.competencia != "" {
		if competencia, err = time.Parse(config.CompetenciaLayout, f.competencia); err != nil {
			return fmt.Errorf("--competencia %q invalida: use o formato AAAA-MM-DD", f.competencia)
		}
	}

	cfg, err := config.Load(f.configPath)
	if err != nil {
		return err
	}
	if err := cfg.ValidateForQuery(); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	busca := buscaMunicipio{
		ctx: cmd.Context(), out: out, errOut: cmd.ErrOrStderr(),
		cache:  ibge.NovoCache(""),
		client: ibge.New(ibge.Config{UserAgent: AppVersion()}),
		origem: "argumento",
	}
	municipio, err := busca.resolver(args[0])
	if err != nil {
		return err
	}

	client, err := newParametrosClient(cmd, cfg, f)
	if err != nil {
		return err
	}

	cache := parametrizacao.NovoCache("")
	defer func() {
		if err := cache.Gravar(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "aviso: nao consegui gravar o cache de parametros: %v\n", err)
		}
	}()
	consulta := parametrizacao.NovaConsulta(client, cache)
	consulta.Renovar = f.semCache

	fmt.Fprintf(out, "Parametros municipais em %s (%s)\n\n", consulta.BaseURL(), cfg.Ambiente)

	respostaConvenio, err := consulta.Convenio(cmd.Context(), municipio.codigo)
	if err != nil {
		return explainParametrosError(err, municipio, codigo, competencia)
	}
	convenio := respostaConvenio.Valor
	origem := origemDasRespostas{}
	origem.anotar(respostaConvenio.DoCache, respostaConvenio.ConsultadoEm)

	fmt.Fprintf(out, "  Municipio    %s\n", municipio)
	fmt.Fprintf(out, "  Convenio     %s\n", descreverConvenio(convenio))
	fmt.Fprintf(out, "  Servico      %s\n", descreverServico(codigo))
	if !competencia.IsZero() {
		fmt.Fprintf(out, "  Competencia  %s\n", competencia.Format(layoutData))
	}

	// The ADN parameterises only municipalities with an active convênio, and
	// answers the rest with a 404 for every rate: asking would only fail.
	if !convenio.Ativo {
		fmt.Fprintf(out, "\nO ADN so parametriza municipios com convenio ativo: nao ha aliquota a consultar.\n")
		orientarAliquota(out, cfg.Ambiente, false)
		origem.informar(out)
		return nil
	}

	var respostaAliquotas parametrizacao.Resposta[[]parametrizacao.Aliquota]
	if competencia.IsZero() {
		respostaAliquotas, err = consulta.HistoricoAliquotas(cmd.Context(), municipio.codigo, codigo)
	} else {
		respostaAliquotas, err = consulta.Aliquota(cmd.Context(), municipio.codigo, codigo, competencia)
	}
	if err != nil {
		return explainParametrosError(err, municipio, codigo, competencia)
	}
	aliquotas := respostaAliquotas.Valor
	origem.anotar(respostaAliquotas.DoCache, respostaAliquotas.ConsultadoEm)

	titulo := "Historico de aliquotas do ISSQN"
	if !competencia.IsZero() {
		titulo = "Aliquota do ISSQN na competencia"
	}
	fmt.Fprintf(out, "\n%s\n", titulo)
	if len(aliquotas) == 0 {
		fmt.Fprintf(out, "  (o ADN respondeu sem nenhum periodo)\n")
	}
	for _, a := range aliquotas {
		fmt.Fprintf(out, "  %-8s  %-26s  incidencia %s\n", formatarPercentual(a.Percentual), descreverVigencia(a), valorOuTraco(a.Incidencia))
	}

	orientarAliquota(out, cfg.Ambiente, true)
	origem.informar(out)
	return nil
}

// origemDasRespostas tracks whether anything shown came from the cache, and
// the oldest such answer, so the output never passes a remembered answer off
// as one the ADN just gave.
type origemDasRespostas struct {
	doCache    bool
	maisAntiga time.Time
}

func (o *origemDasRespostas) anotar(doCache bool, consultadoEm time.Time) {
	if !doCache {
		return
	}
	if !o.doCache || consultadoEm.Before(o.maisAntiga) {
		o.maisAntiga = consultadoEm
	}
	o.doCache = true
}

func (o *origemDasRespostas) informar(out io.Writer) {
	if !o.doCache {
		return
	}
	fmt.Fprintf(out, "\nResposta guardada, obtida do ADN em %s. Use --sem-cache para consultar de novo.\n",
		o.maisAntiga.Local().Format(layoutDataHora))
}

// newParametrosClient builds a client authenticated by the configured
// certificate, in the configured environment.
func newParametrosClient(cmd *cobra.Command, cfg *config.Config, f *parametrosFlags) (*parametrizacao.Client, error) {
	certInfo, err := loadCertificate(cmd, cfg, &emitirFlags{
		configPath: f.configPath,
		certPath:   f.certPath,
		password:   f.password,
	})
	if err != nil {
		return nil, err
	}

	tlsCert, err := certInfo.TLSCertificate()
	if err != nil {
		return nil, fmt.Errorf("certificado nao pode ser usado na conexao: %w", err)
	}

	return newParametrizacaoClient(parametrizacao.Config{
		Ambiente:    cfg.Ambiente,
		Certificate: tlsCert,
		OnRetry: func(wait time.Duration, _ error) {
			fmt.Fprintf(cmd.ErrOrStderr(), "O ADN nao respondeu; tentando de novo em %s...\n", wait)
		},
	})
}

func descreverConvenio(c *parametrizacao.Convenio) string {
	if !c.Ativo {
		return "INATIVO — " + valorOuTraco(c.Mensagem)
	}
	return fmt.Sprintf("ativo (emissor nacional: %s, ambiente nacional: %s)",
		c.AderenteEmissorNacional, c.AderenteAmbienteNacional)
}

func descreverServico(codigo string) string {
	if s, ok := servico.PorCodigo(codigo[:8]); ok {
		return codigo + "  " + s.Descricao
	}
	return codigo
}

func descreverVigencia(a parametrizacao.Aliquota) string {
	if a.Fim == nil {
		return "de " + a.Inicio.Format(layoutData) + " em diante"
	}
	return "de " + a.Inicio.Format(layoutData) + " a " + a.Fim.Format(layoutData)
}

// formatarPercentual writes a rate the way iss_aliquota takes it, with the
// Brazilian decimal comma for reading.
func formatarPercentual(p *float64) string {
	if p == nil {
		return "(vazia)"
	}
	return strings.Replace(fmt.Sprintf("%.2f%%", *p), ".", ",", 1)
}

func valorOuTraco(valor string) string {
	if strings.TrimSpace(valor) == "" {
		return "-"
	}
	return valor
}

// orientarAliquota says what the answer means for iss_aliquota. The rules are
// E0635 and E0640 of ANEXO I: among the Simples providers this emitter serves,
// they are the ones the convênio decides (ADR 0006). An answer from the
// testing environment says so, because its data is made up.
func orientarAliquota(out io.Writer, ambiente string, ativo bool) {
	if ativo {
		fmt.Fprintf(out, "\nCom o convenio ativo, quem apura o ISSQN pela aliquota do municipio nao a informa:\n"+
			"a Sefin aplica a parametrizada. Um ME/EPP com regime_apuracao iss-municipio ou\n"+
			"fora-do-sn deixa iss_aliquota em 0, ou a Sefin rejeita (E0635).\n")
	} else {
		fmt.Fprintf(out, "\nNeste municipio, um ME/EPP com regime_apuracao iss-municipio ou fora-do-sn\n"+
			"precisa informar iss_aliquota, ou a Sefin rejeita (E0640). Confirme a aliquota\n"+
			"com a prefeitura.\n")
	}

	if ambiente != config.EnvProducao {
		fmt.Fprintf(out, "\nAtencao: em %s os dados sao de teste. O que vale e a resposta de producao.\n", ambiente)
	}
}

// explainParametrosError turns the client's sentinels into advice.
func explainParametrosError(err error, municipio municipioInformado, codigo string, competencia time.Time) error {
	switch {
	case errors.Is(err, parametrizacao.ErrNaoEncontrado):
		quando := "em nenhuma data"
		if !competencia.IsZero() {
			quando = "na competencia " + competencia.Format(layoutData)
		}
		return fmt.Errorf("%w\n%s nao tem aliquota parametrizada para %s %s. Se o municipio criou um "+
			"codigo proprio abaixo deste servico, informe-o com --complemento; se nao, confirme a "+
			"aliquota com a prefeitura", err, municipio, codigo, quando)
	case errors.Is(err, parametrizacao.ErrAcessoNegado):
		return fmt.Errorf("%w\nConfira se o certificado e um A1 valido da ICP-Brasil: o ADN nao aceita "+
			"certificado de teste autoassinado", err)
	case errors.Is(err, parametrizacao.ErrInacessivel), errors.Is(err, parametrizacao.ErrIndisponivel):
		return fmt.Errorf("%w\nConfira a conexao e tente de novo em alguns minutos; se persistir, "+
			"o servico do governo pode estar fora do ar", err)
	default:
		return err
	}
}
