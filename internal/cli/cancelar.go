package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/edusouza/nfse-emissor-go/internal/config"
	"github.com/edusouza/nfse-emissor-go/internal/domain/query"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/sefin"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/xmlsigner"
	"github.com/edusouza/nfse-emissor-go/pkg/xmlbuilder"
)

// cancelReasons maps the flag values a person types onto the schema's codes.
var cancelReasons = map[string]string{
	"erro-emissao": xmlbuilder.CancelReasonIssuingError,
	"nao-prestado": xmlbuilder.CancelReasonServiceNotProvided,
	"outros":       xmlbuilder.CancelReasonOther,
}

type cancelarFlags struct {
	configPath    string
	certPath      string
	password      string
	outputDir     string
	motivo        string
	justificativa string
	confirmar     bool
	sobrescrever  bool
}

func newCancelarCommand() *cobra.Command {
	var f cancelarFlags

	cmd := &cobra.Command{
		Use:   "cancelar <chave-de-acesso>",
		Short: "Cancela uma NFS-e emitida",
		Long: `Registra um evento de cancelamento para uma NFS-e ja emitida.

O cancelamento e um documento a parte: um pedido de registro de evento,
assinado com o mesmo certificado e enviado a Sefin, que o valida e gera o
evento vinculado a nota.

A justificativa entra no registro fiscal e o schema exige ao menos 15
caracteres — descreva o que aconteceu, nao apenas "erro".

Cancelar uma nota com valor fiscal pede confirmacao no terminal.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCancelar(cmd, args[0], &f)
		},
	}

	fl := cmd.Flags()
	fl.StringVarP(&f.configPath, "config", "c", config.DefaultFileName, "arquivo de configuracao")
	fl.StringVar(&f.certPath, "cert", "", "certificado PFX/P12 (sobrescreve o da configuracao)")
	fl.StringVarP(&f.password, "senha", "s", "", "senha do certificado (prefira "+envCertPassword+")")
	fl.StringVarP(&f.outputDir, "saida", "o", "", "diretorio onde gravar o evento")
	fl.StringVarP(&f.motivo, "motivo", "m", "", "erro-emissao | nao-prestado | outros (obrigatorio)")
	fl.StringVarP(&f.justificativa, "justificativa", "j", "", "explicacao do cancelamento (15 a 255 caracteres)")
	fl.BoolVar(&f.confirmar, "confirmar", false, "dispensa a confirmacao interativa")
	fl.BoolVar(&f.sobrescrever, "sobrescrever", false, "substitui um arquivo ja existente")

	return cmd
}

func runCancelar(cmd *cobra.Command, chave string, f *cancelarFlags) error {
	if err := query.ValidateAccessKey(chave); err != nil {
		return err
	}

	codigo, ok := cancelReasons[f.motivo]
	if !ok {
		return fmt.Errorf("informe --motivo: erro-emissao, nao-prestado ou outros")
	}

	cfg, err := config.Load(f.configPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	built, err := xmlbuilder.BuildCancellation(xmlbuilder.CancellationConfig{
		Environment:        cfg.EnvironmentCode(),
		ApplicationVersion: AppVersion(),
		AuthorCNPJ:         cfg.Prestador.CNPJ,
		AccessKey:          chave,
		ReasonCode:         codigo,
		Reason:             f.justificativa,
	})
	if err != nil {
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

	signed, err := xmlsigner.NewXMLSigner(certInfo).SignEventRequest(built.XML)
	if err != nil {
		return fmt.Errorf("falha ao assinar o pedido de cancelamento: %w", err)
	}

	if err := confirmCancellation(cmd, cfg, f, chave); err != nil {
		return err
	}

	tlsCert, err := certInfo.TLSCertificate()
	if err != nil {
		return fmt.Errorf("certificado nao pode ser usado na conexao: %w", err)
	}
	client, err := newSefinClient(sefin.Config{Environment: cfg.Ambiente, Certificate: tlsCert})
	if err != nil {
		return err
	}

	result, err := client.RegisterEvent(cmd.Context(), chave, []byte(signed))
	if err != nil {
		return explainQueryError(err)
	}

	path, err := writeEvent(cfg, f, built.RequestID, result)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Cancelamento registrado\n")
	fmt.Fprintf(out, "  NFS-e            %s\n", chave)
	fmt.Fprintf(out, "  Pedido           %s\n", built.RequestID)
	fmt.Fprintf(out, "  Ambiente         %s\n", sefin.EnvironmentName(result.EnvironmentCode))
	fmt.Fprintf(out, "  Processado em    %s\n", result.ProcessedAt.Local().Format("02/01/2006 15:04:05"))
	fmt.Fprintf(out, "  Evento           %s\n", path)
	return nil
}

// confirmCancellation asks before cancelling an invoice that carries fiscal
// value. A cancellation is itself a fiscal act and cannot be undone.
func confirmCancellation(cmd *cobra.Command, cfg *config.Config, f *cancelarFlags, chave string) error {
	if cfg.Ambiente != config.EnvProducao || f.confirmar {
		return nil
	}

	in, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(in.Fd())) {
		return fmt.Errorf("cancelamento em producao sem terminal para confirmar: use --confirmar se e isso mesmo que voce quer")
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\nVoce esta prestes a CANCELAR uma NFS-e COM VALOR FISCAL.\n")
	fmt.Fprintf(out, "  Nota           %s\n", chave)
	fmt.Fprintf(out, "  Justificativa  %s\n", f.justificativa)
	fmt.Fprintf(out, "\nO cancelamento e definitivo. Confirmar? [s/N] ")

	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil {
		return fmt.Errorf("falha ao ler a confirmacao: %w", err)
	}
	if r := strings.ToLower(strings.TrimSpace(answer)); r != "s" && r != "sim" {
		return fmt.Errorf("cancelamento abortado")
	}
	return nil
}

func writeEvent(cfg *config.Config, f *cancelarFlags, requestID string, result *sefin.EventResult) (string, error) {
	dir := f.outputDir
	if dir == "" {
		dir = cfg.Saida.Diretorio
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("nao foi possivel criar o diretorio de saida: %w", err)
	}

	path := filepath.Join(dir, requestID+"-evento.xml")
	if err := writeNew(path, result.EventXML, f.sobrescrever); err != nil {
		return "", fmt.Errorf("o cancelamento foi registrado mas o evento nao pode ser gravado: %w", err)
	}
	return path, nil
}
