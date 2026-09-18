package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edusouza/nfse-emissor-go/internal/config"
	"github.com/edusouza/nfse-emissor-go/internal/domain/query"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/sefin"
)

type consultarFlags struct {
	configPath string
	certPath   string
	password   string
	outputDir  string

	dpsID         string
	somenteExiste bool
	sobrescrever  bool
}

func newConsultarCommand() *cobra.Command {
	var f consultarFlags

	cmd := &cobra.Command{
		Use:   "consultar [chave-de-acesso]",
		Short: "Consulta uma NFS-e ja emitida",
		Long: `Busca na Sefin Nacional uma NFS-e pela chave de acesso e grava o XML.

Com --dps, busca pelo identificador da declaracao em vez da chave — util
quando uma emissao foi interrompida e voce nao sabe se a nota chegou a ser
gerada. Nesse caso, --existe responde apenas sim ou nao, o que o governo
atende para qualquer certificado valido; ja a chave de acesso so e informada
a quem consta na nota (prestador, tomador ou intermediario).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConsultar(cmd, args, &f)
		},
	}

	fl := cmd.Flags()
	fl.StringVarP(&f.configPath, "config", "c", config.DefaultFileName, "arquivo de configuracao")
	fl.StringVar(&f.certPath, "cert", "", "certificado PFX/P12 (sobrescreve o da configuracao)")
	fl.StringVarP(&f.password, "senha", "s", "", "senha do certificado (prefira "+envCertPassword+")")
	fl.StringVarP(&f.outputDir, "saida", "o", "", "diretorio onde gravar o XML")
	fl.StringVar(&f.dpsID, "dps", "", "consulta pelo identificador da DPS em vez da chave de acesso")
	fl.BoolVar(&f.somenteExiste, "existe", false, "com --dps, apenas informa se a NFS-e foi gerada")
	fl.BoolVar(&f.sobrescrever, "sobrescrever", false, "substitui um arquivo ja existente")

	return cmd
}

func runConsultar(cmd *cobra.Command, args []string, f *consultarFlags) error {
	if len(args) == 0 && f.dpsID == "" {
		return fmt.Errorf("informe a chave de acesso ou use --dps")
	}
	if len(args) > 0 && f.dpsID != "" {
		return fmt.Errorf("informe a chave de acesso ou --dps, nunca os dois")
	}
	if f.somenteExiste && f.dpsID == "" {
		return fmt.Errorf("--existe so vale junto de --dps")
	}

	cfg, err := config.Load(f.configPath)
	if err != nil {
		return err
	}
	// A lookup needs the environment and a certificate, not a full emitter
	// configuration.
	if err := cfg.ValidateForQuery(); err != nil {
		return err
	}

	client, err := newQueryClient(cmd, cfg, f)
	if err != nil {
		return err
	}

	if f.dpsID != "" {
		return consultarPorDPS(cmd, cfg, f, client)
	}
	return consultarPorChave(cmd, cfg, f, client, args[0])
}

// newQueryClient builds a client authenticated by the configured certificate.
func newQueryClient(cmd *cobra.Command, cfg *config.Config, f *consultarFlags) (*sefin.Client, error) {
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

	return newSefinClient(sefin.Config{Environment: cfg.Ambiente, Certificate: tlsCert})
}

func consultarPorChave(cmd *cobra.Command, cfg *config.Config, f *consultarFlags, client *sefin.Client, chave string) error {
	// Catch a malformed key before spending a round-trip on it.
	if err := query.ValidateAccessKey(chave); err != nil {
		return err
	}

	result, err := client.FetchNFSe(cmd.Context(), chave)
	if err != nil {
		return explainQueryError(err)
	}

	path, err := writeQueriedNFSe(cfg, f, result)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "NFS-e encontrada\n")
	fmt.Fprintf(out, "  Chave de acesso  %s\n", result.AccessKey)
	fmt.Fprintf(out, "  Ambiente         %s\n", sefin.EnvironmentName(result.EnvironmentCode))
	fmt.Fprintf(out, "  Consultada em    %s\n", result.ProcessedAt.Local().Format("02/01/2006 15:04:05"))
	fmt.Fprintf(out, "  Arquivo          %s\n", path)
	return nil
}

func consultarPorDPS(cmd *cobra.Command, cfg *config.Config, f *consultarFlags, client *sefin.Client) error {
	out := cmd.OutOrStdout()

	if f.somenteExiste {
		existe, err := client.DPSExists(cmd.Context(), f.dpsID)
		if err != nil {
			return explainQueryError(err)
		}
		if !existe {
			// Not an error: "no invoice was issued" is a valid, useful answer.
			fmt.Fprintf(out, "Nenhuma NFS-e foi gerada para a DPS %s.\n", f.dpsID)
			return nil
		}
		fmt.Fprintf(out, "A DPS %s gerou uma NFS-e.\n", f.dpsID)
		fmt.Fprintf(out, "Use 'nfse consultar --dps %s' para obter a chave de acesso.\n", f.dpsID)
		return nil
	}

	lookup, err := client.LookupDPS(cmd.Context(), f.dpsID)
	if err != nil {
		return explainQueryError(err)
	}

	fmt.Fprintf(out, "NFS-e gerada a partir da DPS\n")
	fmt.Fprintf(out, "  DPS              %s\n", lookup.DPSID)
	fmt.Fprintf(out, "  Chave de acesso  %s\n", lookup.AccessKey)
	fmt.Fprintf(out, "  Ambiente         %s\n", sefin.EnvironmentName(lookup.EnvironmentCode))
	fmt.Fprintf(out, "\nBaixe o XML com:\n  nfse consultar %s\n", lookup.AccessKey)
	return nil
}

// explainQueryError turns the client's sentinels into advice.
//
// It appends whatever detail the client attached rather than replacing the
// message: the client distinguishes a refusal that came from the government
// from one produced by something in between, and losing that distinction sends
// the user looking in the wrong place.
func explainQueryError(err error) error {
	switch {
	case errors.Is(err, sefin.ErrNotFound):
		return fmt.Errorf("nao encontrado na Sefin Nacional: confira a chave e o ambiente da configuracao")
	case errors.Is(err, sefin.ErrForbidden):
		msg := "consulta negada: por sigilo fiscal, so o prestador, o tomador ou o " +
			"intermediario da nota podem consulta-la, e o certificado usado precisa ser o de um deles"
		if detail := detailBeyond(err, sefin.ErrForbidden); detail != "" {
			msg += "\n" + detail
		}
		return errors.New(msg)
	default:
		return err
	}
}

// detailBeyond returns the part of err's message that the sentinel alone does
// not account for.
func detailBeyond(err error, sentinel error) string {
	full, bare := err.Error(), sentinel.Error()
	if full == bare {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(full, bare))
}

func writeQueriedNFSe(cfg *config.Config, f *consultarFlags, result *sefin.NFSeResult) (string, error) {
	dir := f.outputDir
	if dir == "" {
		dir = cfg.Saida.Diretorio
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("nao foi possivel criar o diretorio de saida: %w", err)
	}

	path := filepath.Join(dir, result.AccessKey+"-nfse.xml")
	if err := writeNew(path, result.NFSeXML, f.sobrescrever); err != nil {
		return "", err
	}
	return path, nil
}
