// Package cli wires the nfse command tree.
package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCommand builds the root command with every subcommand attached.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "nfse",
		Short: "Emissor de NFS-e em linha de comando",
		Long: `nfse emite Notas Fiscais de Servico eletronicas pelo Sistema Nacional NFS-e.

O fluxo de uma emissao tem quatro etapas: montar o XML da DPS a partir dos seus
dados, validar, assinar com o certificado digital A1 e enviar para a Sefin
Nacional. Cada etapa pode ser executada isoladamente.`,
		// Errors are printed once by main; usage on a runtime error is noise.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newVersionCommand(),
		newCertCommand(),
	)

	return root
}

// Execute runs the root command against os.Args.
func Execute() error {
	return NewRootCommand().Execute()
}
