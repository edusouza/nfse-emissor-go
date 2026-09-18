package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/edusouza/nfse-emissor-go/internal/config"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Gerencia o arquivo de configuracao",
	}
	cmd.AddCommand(newConfigInitCommand(), newConfigCheckCommand())
	return cmd
}

func newConfigInitCommand() *cobra.Command {
	var (
		path  string
		force bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Cria um arquivo de configuracao comentado",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !force {
				if _, err := os.Stat(path); err == nil {
					return fmt.Errorf("%s ja existe; use --forcar para sobrescrever", path)
				} else if !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}

			if err := os.WriteFile(path, []byte(config.ExampleFile), 0o644); err != nil {
				return fmt.Errorf("nao foi possivel gravar %q: %w", path, err)
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"%s criado.\n\nPreencha os campos marcados como OBRIGATORIO e depois rode:\n  nfse config check\n",
				path)
			return nil
		},
	}

	cmd.Flags().StringVarP(&path, "arquivo", "a", config.DefaultFileName, "caminho do arquivo a criar")
	cmd.Flags().BoolVar(&force, "forcar", false, "sobrescreve um arquivo existente")

	return cmd
}

func newConfigCheckCommand() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Verifica se a configuracao esta completa e coerente",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s esta valido.\n\n", path)
			fmt.Fprintf(out, "  Ambiente    %s\n", cfg.Ambiente)
			fmt.Fprintf(out, "  Prestador   %s (%s)\n", cfg.Prestador.Nome, cfg.Prestador.CNPJ)
			fmt.Fprintf(out, "  Regime      %s\n", cfg.Prestador.RegimeTributario)
			fmt.Fprintf(out, "  Municipio   %s\n", cfg.Prestador.Municipio)
			fmt.Fprintf(out, "  Serie       %s\n", cfg.DPS.Serie)

			if cfg.Ambiente == config.EnvProducao {
				fmt.Fprintf(out, "\nAtencao: ambiente de producao. As notas emitidas terao valor fiscal.\n")
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&path, "arquivo", "a", config.DefaultFileName, "arquivo de configuracao")
	return cmd
}
