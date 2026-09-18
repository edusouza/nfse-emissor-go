package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/edusouza/nfse-emissor-go/internal/config"
	"github.com/edusouza/nfse-emissor-go/internal/domain/validation"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/sefin"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/xmlsigner"
	"github.com/edusouza/nfse-emissor-go/pkg/cnpjcpf"
	"github.com/edusouza/nfse-emissor-go/pkg/xmlbuilder"
)

// emitirFlags collects the command-line overrides for a single invoice.
type emitirFlags struct {
	configPath string
	notaPath   string
	certPath   string
	password   string
	outputDir  string

	numero      string
	competencia string
	descricao   string
	codigo      string

	valor                  float64
	descontoIncondicionado float64
	descontoCondicionado   float64
	deducoes               float64
	issAliquota            float64

	retencao     string
	tomadorCNPJ  string
	tomadorCPF   string
	tomadorNome  string
	tomadorEmail string

	semAssinar   bool
	enviar       bool
	confirmar    bool
	sobrescrever bool
}

func newEmitirCommand() *cobra.Command {
	var f emitirFlags

	cmd := &cobra.Command{
		Use:   "emitir",
		Short: "Monta, valida e assina o XML de uma DPS",
		Long: `Monta a DPS a partir da sua configuracao, valida os dados e assina com o
certificado A1, gravando o XML pronto para envio.

Os dados vem de tres lugares, cada um sobrescrevendo o anterior:

  1. a secao "padroes" do nfse.yaml, para o que se repete em toda nota;
  2. o arquivo passado em --yaml, quando houver;
  3. as flags desta linha de comando.

Assim, quem sempre emite o mesmo tipo de servico so precisa informar o valor:

  nfse emitir --numero 42 --valor 1500 --descricao "Consultoria - agosto/2026"

Por padrao o comando para na assinatura, gravando o XML. Com --enviar ele
transmite a DPS para a Sefin Nacional e grava a NFS-e autorizada.

Emitir com ambiente "producao" gera uma nota com valor fiscal e pede
confirmacao no terminal; use --confirmar para dispensa-la em scripts.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runEmitir(cmd, &f)
		},
	}

	fl := cmd.Flags()
	fl.StringVarP(&f.configPath, "config", "c", config.DefaultFileName, "arquivo de configuracao")
	fl.StringVarP(&f.notaPath, "yaml", "y", "", "arquivo YAML com os dados da nota")
	fl.StringVar(&f.certPath, "cert", "", "certificado PFX/P12 (sobrescreve o da configuracao)")
	fl.StringVarP(&f.password, "senha", "s", "", "senha do certificado (prefira "+envCertPassword+")")
	fl.StringVarP(&f.outputDir, "saida", "o", "", "diretorio onde gravar o XML")

	fl.StringVarP(&f.numero, "numero", "n", "", "numero da DPS (padrao: o proximo da serie)")
	fl.StringVar(&f.competencia, "competencia", "", "data de competencia (AAAA-MM-DD; padrao: hoje)")
	fl.StringVarP(&f.descricao, "descricao", "d", "", "descricao do servico prestado")
	fl.StringVar(&f.codigo, "codigo-servico", "", "codigo de tributacao nacional (6 digitos)")

	fl.Float64VarP(&f.valor, "valor", "v", 0, "valor do servico")
	fl.Float64Var(&f.descontoIncondicionado, "desconto-incondicionado", 0, "desconto incondicionado")
	fl.Float64Var(&f.descontoCondicionado, "desconto-condicionado", 0, "desconto condicionado")
	fl.Float64Var(&f.deducoes, "deducoes", 0, "deducoes permitidas")
	fl.Float64Var(&f.issAliquota, "iss-aliquota", 0, "aliquota de ISS em porcentagem")
	fl.StringVar(&f.retencao, "retencao", "", "retencao do ISSQN: nao | tomador | intermediario")

	fl.StringVar(&f.tomadorCNPJ, "tomador-cnpj", "", "CNPJ do tomador")
	fl.StringVar(&f.tomadorCPF, "tomador-cpf", "", "CPF do tomador")
	fl.StringVar(&f.tomadorNome, "tomador-nome", "", "nome do tomador")
	fl.StringVar(&f.tomadorEmail, "tomador-email", "", "e-mail do tomador")

	fl.BoolVar(&f.semAssinar, "sem-assinar", false, "gera o XML sem assinar (para inspecao; nao serve para envio)")
	fl.BoolVar(&f.enviar, "enviar", false, "envia a DPS assinada para a Sefin Nacional e grava a NFS-e")
	fl.BoolVar(&f.confirmar, "confirmar", false, "dispensa a confirmacao interativa ao emitir em producao")
	fl.BoolVar(&f.sobrescrever, "sobrescrever", false, "substitui um arquivo ja existente com o mesmo identificador")

	return cmd
}

func runEmitir(cmd *cobra.Command, f *emitirFlags) error {
	cfg, err := config.Load(f.configPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	if f.enviar && f.semAssinar {
		return fmt.Errorf("--enviar e --sem-assinar se excluem: a Sefin so aceita uma DPS assinada")
	}

	statePath := config.StatePath(f.configPath)
	state, err := config.LoadState(statePath)
	if err != nil {
		return err
	}

	nota, err := resolveNota(cmd, cfg, f, state)
	if err != nil {
		return err
	}
	if err := nota.Validate(); err != nil {
		return err
	}

	// Check the ISS rate against the government's own rules before building
	// anything: a rate that is forbidden or missing is a certain rejection, and
	// the error code alone would not say what to do about it.
	if err := validation.ValidateISSRate(issRateContext(cfg, nota)); err != nil {
		return err
	}

	built, err := buildDPS(cfg, nota)
	if err != nil {
		return err
	}

	// Validate the generated XML before spending the certificate on it: a
	// structural problem is cheaper to report here than as a rejection later.
	if errs := validation.NewStructuralValidator().ValidateDPS(built.XML); len(errs) > 0 {
		var b strings.Builder
		b.WriteString("o XML gerado nao passou na validacao estrutural:")
		for _, e := range errs {
			fmt.Fprintf(&b, "\n  - %s", e.Error())
		}
		return fmt.Errorf("%s", b.String())
	}

	if f.semAssinar {
		path, err := writeDPS(cfg, f, built.DPSID, built.XML, false)
		if err != nil {
			return err
		}
		return report(cmd, cfg, nota, built.DPSID, path, false)
	}

	certInfo, err := loadCertificate(cmd, cfg, f)
	if err != nil {
		return err
	}

	if err := ensureCertificateBelongsToProvider(certInfo, cfg.Prestador.CNPJ); err != nil {
		return err
	}

	signedXML, err := signDPS(certInfo, built.XML)
	if err != nil {
		return err
	}

	dpsPath, err := writeDPS(cfg, f, built.DPSID, signedXML, true)
	if err != nil {
		return err
	}

	// Record only after the file exists. Advancing the counter for a document
	// that was never written would skip a number for no reason.
	state.Record(cfg.DPS.Serie, nota.Numero)
	if err := state.Save(statePath); err != nil {
		return err
	}

	if !f.enviar {
		return report(cmd, cfg, nota, built.DPSID, dpsPath, true)
	}

	// Issuing in production creates a document with fiscal value, which can
	// only be undone through a cancellation procedure. Ask first.
	if err := confirmProduction(cmd, cfg, f, nota); err != nil {
		return err
	}

	result, err := transmit(cmd.Context(), cfg, certInfo, signedXML)
	if err != nil {
		return err
	}

	nfsePath, err := writeNFSe(cfg, f, result)
	if err != nil {
		return err
	}

	return reportEmission(cmd, nota, built.DPSID, dpsPath, nfsePath, result)
}

// confirmProduction asks the user to confirm an emission that carries fiscal
// value, unless --confirmar was given or there is no terminal to ask on.
func confirmProduction(cmd *cobra.Command, cfg *config.Config, f *emitirFlags, nota config.Nota) error {
	if cfg.Ambiente != config.EnvProducao || f.confirmar {
		return nil
	}

	in, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(in.Fd())) {
		return fmt.Errorf("emissao em producao sem terminal para confirmar: use --confirmar se e isso mesmo que voce quer")
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\nVoce esta prestes a emitir uma NFS-e COM VALOR FISCAL.\n")
	fmt.Fprintf(out, "  Prestador  %s\n", cfg.Prestador.Nome)
	fmt.Fprintf(out, "  Valor      R$ %.2f\n", nota.Valores.ValorServico)
	fmt.Fprintf(out, "  Servico    %s\n", nota.Servico.Descricao)
	fmt.Fprintf(out, "\nCancelar uma nota emitida exige um pedido de evento. Confirmar? [s/N] ")

	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil {
		return fmt.Errorf("falha ao ler a confirmacao: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "s", "sim":
		return nil
	default:
		return fmt.Errorf("emissao cancelada")
	}
}

// resolveNota layers the config defaults, the --yaml file and the flags, and
// supplies a DPS number when none of them did.
func resolveNota(cmd *cobra.Command, cfg *config.Config, f *emitirFlags, state *config.State) (config.Nota, error) {
	nota := cfg.Padroes

	if f.notaPath != "" {
		fromFile, err := config.LoadNota(f.notaPath)
		if err != nil {
			return config.Nota{}, err
		}
		nota = nota.Merge(*fromFile)
	}

	nota = nota.Merge(notaFromFlags(cmd, f))

	// Remembering the last number is what turns "what number am I on?" from
	// the user's problem into the tool's.
	if nota.Numero == "" {
		nota.Numero = state.NextNumber(cfg.DPS.Serie)
	}

	return nota, nil
}

// notaFromFlags builds an override containing only the flags actually given,
// so that an unused flag's zero value never wipes out a configured default.
func notaFromFlags(cmd *cobra.Command, f *emitirFlags) config.Nota {
	fl := cmd.Flags()
	var override config.Nota

	if fl.Changed("numero") {
		override.Numero = f.numero
	}
	if fl.Changed("competencia") {
		override.Competencia = f.competencia
	}
	if fl.Changed("descricao") {
		override.Servico.Descricao = f.descricao
	}
	if fl.Changed("codigo-servico") {
		override.Servico.CodigoTributacaoNacional = f.codigo
	}

	if fl.Changed("valor") {
		override.Valores.ValorServico = f.valor
	}
	if fl.Changed("desconto-incondicionado") {
		override.Valores.DescontoIncondicionado = f.descontoIncondicionado
	}
	if fl.Changed("desconto-condicionado") {
		override.Valores.DescontoCondicionado = f.descontoCondicionado
	}
	if fl.Changed("deducoes") {
		override.Valores.Deducoes = f.deducoes
	}
	if fl.Changed("iss-aliquota") {
		rate := f.issAliquota
		override.Valores.ISSAliquota = &rate
	}
	if fl.Changed("retencao") {
		override.Valores.RetencaoISSQN = f.retencao
	}

	if fl.Changed("tomador-cnpj") || fl.Changed("tomador-cpf") ||
		fl.Changed("tomador-nome") || fl.Changed("tomador-email") {
		override.Tomador = &config.Tomador{
			CNPJ:  f.tomadorCNPJ,
			CPF:   f.tomadorCPF,
			Nome:  f.tomadorNome,
			Email: f.tomadorEmail,
		}
	}

	return override
}

// buildDPS translates the resolved settings into the XML builder's shape.
func buildDPS(cfg *config.Config, nota config.Nota) (*xmlbuilder.DPSBuildResult, error) {
	competencia, err := nota.CompetenciaDate()
	if err != nil {
		return nil, err
	}

	municipioPrestacao := nota.Servico.MunicipioPrestacao
	if municipioPrestacao == "" {
		municipioPrestacao = cfg.Prestador.Municipio
	}

	dpsCfg := xmlbuilder.DPSConfig{
		Environment:        cfg.EnvironmentCode(),
		EmissionDateTime:   time.Now(),
		ApplicationVersion: AppVersion(),
		Series:             cfg.DPS.Serie,
		Number:             nota.Numero,
		CompetenceDate:     competencia,
		EmitterType:        xmlbuilder.EmitterTypeProvider,
		MunicipalityCode:   cfg.Prestador.Municipio,
		// Substitution stays nil: this is an ordinary emission, not a replacement.
		Provider: xmlbuilder.DPSProvider{
			CNPJ:                  cfg.Prestador.CNPJ,
			Name:                  cfg.Prestador.Nome,
			TaxRegime:             cfg.Prestador.RegimeTributario,
			MunicipalRegistration: cfg.Prestador.InscricaoMunicipal,
			SimplesApuracao:       cfg.RegimeApuracaoCode(),
		},
		Taker: takerFor(nota),
		Service: xmlbuilder.DPSService{
			NationalCode:     nota.Servico.CodigoTributacaoNacional,
			Description:      nota.Servico.Descricao,
			MunicipalityCode: municipioPrestacao,
		},
		Values: xmlbuilder.DPSValues{
			ServiceValue:          nota.Valores.ValorServico,
			UnconditionalDiscount: nota.Valores.DescontoIncondicionado,
			ConditionalDiscount:   nota.Valores.DescontoCondicionado,
			Deductions:            nota.Valores.Deducoes,
			ISSRate:               nota.ISSRate(),
			ISSRetention:          nota.RetencaoCode(),
		},
	}

	built, err := xmlbuilder.NewDPSBuilder(dpsCfg).Build()
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel montar o XML da DPS: %w", err)
	}
	return built, nil
}

// takerFor returns the taker element, or nil when the invoice has none.
func takerFor(nota config.Nota) *xmlbuilder.DPSTaker {
	t := nota.Tomador
	if t == nil || t.NaoIdentificado {
		return nil
	}
	return &xmlbuilder.DPSTaker{
		CNPJ:  t.CNPJ,
		CPF:   t.CPF,
		Name:  t.Nome,
		Email: t.Email,
	}
}

// ensureCertificateBelongsToProvider refuses a certificate issued to someone
// other than the provider named in the document.
//
// The Sefin identifies the issuer by the certificate on the TLS connection, so
// a mismatch comes back as a bare 403 — no envelope, no explanation, and a
// status a corporate proxy produces too. Both numbers are on this machine
// before any connection is opened, so the check belongs here.
//
// It runs before signing, not just before sending: a DPS signed with the wrong
// certificate cannot be salvaged by resending it with the right one, because
// the signature inside the XML is part of what the government validates.
//
// An unreadable certificate CNPJ is not an error. Only ICP-Brasil guarantees
// the "RAZAO SOCIAL:CNPJ" form, and refusing everything else would break
// anyone whose certificate is laid out differently.
func ensureCertificateBelongsToProvider(certInfo *xmlsigner.CertificateInfo, providerCNPJ string) error {
	certCNPJ := certInfo.SubjectCNPJ()
	if certCNPJ == "" || providerCNPJ == "" {
		return nil
	}
	if certCNPJ == cnpjcpf.CleanCNPJ(providerCNPJ) {
		return nil
	}

	return fmt.Errorf("o certificado nao e do prestador desta nota:\n"+
		"  Certificado  %s (%s)\n"+
		"  Prestador    %s\n"+
		"Use o A1 da empresa que esta emitindo. O certificado de teste de exemplos/ "+
		"serve para montar e assinar offline, nunca para enviar",
		formatCNPJ(certCNPJ), certInfo.SubjectHolderName(), formatCNPJ(cnpjcpf.CleanCNPJ(providerCNPJ)))
}

// formatCNPJ renders a CNPJ the way a person reads it, so that two numbers
// side by side can be compared at a glance.
func formatCNPJ(cnpj string) string {
	if len(cnpj) != 14 {
		return cnpj
	}
	return cnpj[0:2] + "." + cnpj[2:5] + "." + cnpj[5:8] + "/" + cnpj[8:12] + "-" + cnpj[12:14]
}

// loadCertificate reads and validates the A1 certificate.
func loadCertificate(cmd *cobra.Command, cfg *config.Config, f *emitirFlags) (*xmlsigner.CertificateInfo, error) {
	certPath := f.certPath
	if certPath == "" {
		certPath = cfg.Certificado.Arquivo
	}
	if certPath == "" {
		return nil, fmt.Errorf("certificado nao informado: preencha certificado.arquivo no %s ou use --cert", f.configPath)
	}

	pass, err := resolveCertPassword(f.password, cmd.Flags().Changed("senha"), cmd.ErrOrStderr())
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel ler o certificado: %w", err)
	}

	certInfo, err := xmlsigner.ParsePFX(data, pass)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel abrir o certificado (senha incorreta ou arquivo invalido): %w", err)
	}

	if err := xmlsigner.NewCertificateValidator().ValidateForSigning(certInfo); err != nil {
		return nil, fmt.Errorf("o certificado nao pode assinar: %w", err)
	}

	return certInfo, nil
}

// signDPS applies the XMLDSig signature.
func signDPS(certInfo *xmlsigner.CertificateInfo, dpsXML string) (string, error) {
	signedXML, err := xmlsigner.NewXMLSigner(certInfo).SignDPS(dpsXML)
	if err != nil {
		return "", fmt.Errorf("falha ao assinar a DPS: %w", err)
	}
	return signedXML, nil
}

// newSefinClient is a seam: tests replace it to reach a stub server instead of
// the government.
var newSefinClient = sefin.New

// transmit sends the signed DPS and returns the authorised invoice.
//
// The same certificate signs the document and authenticates the connection:
// the national system identifies the issuer by the client certificate.
func transmit(ctx context.Context, cfg *config.Config, certInfo *xmlsigner.CertificateInfo, signedDPS string) (*sefin.EmissionResult, error) {
	tlsCert, err := certInfo.TLSCertificate()
	if err != nil {
		return nil, fmt.Errorf("certificado nao pode ser usado na conexao: %w", err)
	}

	client, err := newSefinClient(sefin.Config{
		Environment: cfg.Ambiente,
		Certificate: tlsCert,
	})
	if err != nil {
		return nil, err
	}

	return client.Emit(ctx, []byte(signedDPS))
}

func writeDPS(cfg *config.Config, f *emitirFlags, dpsID, content string, signed bool) (string, error) {
	dir := f.outputDir
	if dir == "" {
		dir = cfg.Saida.Diretorio
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("nao foi possivel criar o diretorio de saida: %w", err)
	}

	suffix := "-dps.xml"
	if !signed {
		suffix = "-dps-sem-assinatura.xml"
	}

	path := filepath.Join(dir, dpsID+suffix)
	if err := writeNew(path, []byte(content), f.sobrescrever); err != nil {
		return "", err
	}
	return path, nil
}

// writeNew writes a file, refusing to replace one that already exists.
//
// The file name is derived from the DPS identifier, which repeats whenever the
// same series and number are reused. Overwriting silently would destroy a
// signed declaration — and, once transmission is involved, the only local
// record of an invoice that exists at the government.
func writeNew(path string, content []byte, overwrite bool) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if overwrite {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	file, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%q ja existe: o numero da DPS provavelmente ja foi usado.\n"+
				"Use outro --numero, ou --sobrescrever se for mesmo para substituir o arquivo", path)
		}
		return fmt.Errorf("nao foi possivel gravar %q: %w", path, err)
	}
	defer file.Close()

	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("nao foi possivel gravar %q: %w", path, err)
	}
	return nil
}

func report(cmd *cobra.Command, cfg *config.Config, nota config.Nota, dpsID, path string, signed bool) error {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "DPS %s\n", dpsID)
	fmt.Fprintf(out, "  Ambiente       %s\n", cfg.Ambiente)
	fmt.Fprintf(out, "  Valor          R$ %.2f\n", nota.Valores.ValorServico)
	fmt.Fprintf(out, "  Servico        %s\n", nota.Servico.Descricao)
	if signed {
		fmt.Fprintf(out, "  Assinatura     aplicada\n")
	} else {
		fmt.Fprintf(out, "  Assinatura     NAO aplicada (--sem-assinar)\n")
	}
	fmt.Fprintf(out, "  Arquivo        %s\n", path)

	if cfg.Ambiente == config.EnvProducaoRestrita {
		fmt.Fprintf(out, "\nAmbiente de producao restrita: esta DPS nao tem valor fiscal.\n")
	}
	if signed {
		fmt.Fprintf(out, "\nA DPS foi assinada mas nao enviada. Use --enviar para transmiti-la.\n")
	}

	return nil
}

// writeNFSe stores the authorised invoice next to the declaration that produced
// it, named by the access key so the two can be matched later.
func writeNFSe(cfg *config.Config, f *emitirFlags, result *sefin.EmissionResult) (string, error) {
	dir := f.outputDir
	if dir == "" {
		dir = cfg.Saida.Diretorio
	}

	name := result.AccessKey
	if name == "" {
		// Never lose an authorised invoice to a missing field.
		name = result.DPSID
	}

	path := filepath.Join(dir, name+"-nfse.xml")
	if err := writeNew(path, result.NFSeXML, f.sobrescrever); err != nil {
		return "", fmt.Errorf("a NFS-e foi emitida mas nao pode ser gravada: %w", err)
	}
	return path, nil
}

// reportEmission prints the outcome of a transmitted emission.
func reportEmission(cmd *cobra.Command, nota config.Nota, dpsID, dpsPath, nfsePath string, result *sefin.EmissionResult) error {
	out := cmd.OutOrStdout()
	env := sefin.EnvironmentName(result.EnvironmentCode)

	fmt.Fprintf(out, "NFS-e emitida\n")
	fmt.Fprintf(out, "  Chave de acesso  %s\n", result.AccessKey)
	fmt.Fprintf(out, "  DPS              %s\n", dpsID)
	fmt.Fprintf(out, "  Ambiente         %s\n", env)
	fmt.Fprintf(out, "  Valor            R$ %.2f\n", nota.Valores.ValorServico)
	fmt.Fprintf(out, "  Processada em    %s\n", result.ProcessedAt.Local().Format("02/01/2006 15:04:05"))
	fmt.Fprintf(out, "  DPS assinada     %s\n", dpsPath)
	fmt.Fprintf(out, "  NFS-e            %s\n", nfsePath)

	for _, w := range result.Warnings {
		fmt.Fprintf(out, "\naviso: %s", w)
	}
	if len(result.Warnings) > 0 {
		fmt.Fprintln(out)
	}

	// The government's own account of the environment is what decides this, not
	// the local configuration.
	if !result.HasFiscalValue() {
		fmt.Fprintf(out, "\nAmbiente de producao restrita: esta nota NAO tem valor fiscal.\n")
	}
	return nil
}

// issRateContext gathers what the ISS rate rules depend on.
func issRateContext(cfg *config.Config, nota config.Nota) validation.ISSRateContext {
	opSimpNac := 3 // ME/EPP
	if cfg.Prestador.RegimeTributario == config.RegimeMEI {
		opSimpNac = 2
	}

	return validation.ISSRateContext{
		OpSimpNac:   opSimpNac,
		RegApTribSN: cfg.RegimeApuracaoCode(),
		TpRetISSQN:  nota.RetencaoCode(),
		Rate:        nota.ISSRate(),
	}
}
