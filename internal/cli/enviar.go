package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/beevik/etree"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/edusouza/nfse-emissor-go/internal/config"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/sefin"
)

type enviarFlags struct {
	configPath   string
	certPath     string
	password     string
	outputDir    string
	confirmar    bool
	sobrescrever bool
}

func newEnviarCommand() *cobra.Command {
	var f enviarFlags

	cmd := &cobra.Command{
		Use:   "enviar <arquivo-dps.xml>",
		Short: "Transmite uma DPS ja assinada para a Sefin Nacional",
		Long: `Envia para a Sefin Nacional um XML de DPS que ja foi gerado e assinado.

Serve ao fluxo de conferir antes de transmitir: 'nfse emitir' sem --enviar
grava a DPS assinada em disco, voce inspeciona o arquivo, e depois manda
exatamente aquele documento.

Exatamente aquele importa. Chamar 'nfse emitir --enviar' de novo geraria um
documento novo, com outro numero de DPS e outro instante de emissao — a nota
que voce conferiu ficaria para tras.

O arquivo precisa estar assinado: a Sefin recusa uma DPS sem assinatura, e
este comando nao assina nada. Para gerar e enviar de uma vez, use
'nfse emitir --enviar'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnviar(cmd, args[0], &f)
		},
	}

	fl := cmd.Flags()
	fl.StringVarP(&f.configPath, "config", "c", config.DefaultFileName, "arquivo de configuracao")
	fl.StringVar(&f.certPath, "cert", "", "certificado PFX/P12 (sobrescreve o da configuracao)")
	fl.StringVarP(&f.password, "senha", "s", "", "senha do certificado (prefira "+envCertPassword+")")
	fl.StringVarP(&f.outputDir, "saida", "o", "", "diretorio onde gravar a NFS-e")
	fl.BoolVar(&f.confirmar, "confirmar", false, "dispensa a confirmacao interativa")
	fl.BoolVar(&f.sobrescrever, "sobrescrever", false, "substitui um arquivo ja existente")

	return cmd
}

func runEnviar(cmd *cobra.Command, path string, f *enviarFlags) error {
	signedXML, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("nao consegui ler %s: %w", path, err)
	}

	dps, err := inspectSignedDPS(string(signedXML), path)
	if err != nil {
		return err
	}

	cfg, err := config.Load(f.configPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
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

	if err := confirmTransmission(cmd, cfg, f, dps); err != nil {
		return err
	}

	result, err := transmit(cmd.Context(), cfg, certInfo, string(signedXML))
	if err != nil {
		return err
	}

	nfsePath, err := writeNFSe(cfg, &emitirFlags{
		outputDir:    f.outputDir,
		sobrescrever: f.sobrescrever,
	}, result)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "NFS-e emitida\n")
	fmt.Fprintf(out, "  Chave de acesso  %s\n", result.AccessKey)
	fmt.Fprintf(out, "  DPS              %s\n", dps.id)
	fmt.Fprintf(out, "  Ambiente         %s\n", sefin.EnvironmentName(result.EnvironmentCode))
	if dps.value != "" {
		fmt.Fprintf(out, "  Valor            R$ %s\n", dps.value)
	}
	fmt.Fprintf(out, "  Processada em    %s\n", result.ProcessedAt.Local().Format("02/01/2006 15:04:05"))
	fmt.Fprintf(out, "  DPS enviada      %s\n", path)
	fmt.Fprintf(out, "  NFS-e            %s\n", nfsePath)

	if !result.HasFiscalValue() {
		fmt.Fprintf(out, "\nAmbiente de producao restrita: esta nota NAO tem valor fiscal.\n")
	}
	return nil
}

// signedDPS is what the file tells us about the document being sent, which is
// all this command knows: it never rebuilds the declaration.
type signedDPS struct {
	id    string
	value string
}

// inspectSignedDPS reads back the identity of a DPS on disk and refuses a file
// that the Sefin would reject on sight.
func inspectSignedDPS(xml, path string) (signedDPS, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		return signedDPS{}, fmt.Errorf("%s nao e um XML valido: %w", path, err)
	}

	inf := doc.FindElement("DPS/infDPS")
	if inf == nil {
		return signedDPS{}, fmt.Errorf("%s nao parece uma DPS: nao encontrei DPS/infDPS. "+
			"Este comando envia a DPS gerada por 'nfse emitir', nao a NFS-e que volta dela", path)
	}

	// An unsigned file is the likely mistake here, because --sem-assinar names
	// its output the same way. Saying so beats a rejection from the government.
	if doc.FindElement("DPS/Signature") == nil {
		return signedDPS{}, fmt.Errorf("%s nao esta assinado: a Sefin so aceita DPS assinada. "+
			"Gere de novo sem --sem-assinar", path)
	}

	dps := signedDPS{id: inf.SelectAttrValue("Id", "")}
	if dps.id == "" {
		return signedDPS{}, fmt.Errorf("%s nao tem o atributo Id em infDPS", path)
	}
	if v := doc.FindElement("DPS/infDPS/valores/vServPrest/vServ"); v != nil {
		dps.value = v.Text()
	}
	return dps, nil
}

// confirmTransmission asks before sending a declaration that will become a
// document with fiscal value.
func confirmTransmission(cmd *cobra.Command, cfg *config.Config, f *enviarFlags, dps signedDPS) error {
	if cfg.Ambiente != config.EnvProducao || f.confirmar {
		return nil
	}

	in, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(in.Fd())) {
		return fmt.Errorf("envio em producao sem terminal para confirmar: use --confirmar se e isso mesmo que voce quer")
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\nVoce esta prestes a emitir uma NFS-e COM VALOR FISCAL.\n")
	fmt.Fprintf(out, "  Prestador  %s\n", cfg.Prestador.Nome)
	fmt.Fprintf(out, "  DPS        %s\n", dps.id)
	if dps.value != "" {
		fmt.Fprintf(out, "  Valor      R$ %s\n", dps.value)
	}
	fmt.Fprintf(out, "\nConfirmar? [s/N] ")

	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil {
		return fmt.Errorf("falha ao ler a confirmacao: %w", err)
	}
	if r := strings.ToLower(strings.TrimSpace(answer)); r != "s" && r != "sim" {
		return fmt.Errorf("envio abortado")
	}
	return nil
}
