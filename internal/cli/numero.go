// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/edusouza/nfse-emissor-go/internal/config"
)

func newNumeroCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "numero",
		Short: "Consulta e ajusta a numeracao das DPS",
		Long: `O nfse guarda, por serie, o ultimo numero de DPS usado nesta maquina, para
que 'nfse emitir' possa seguir a sequencia sozinho.

O arquivo e uma conveniencia local, nao a verdade: quem decide o que foi
realmente emitido e a Sefin. Emitir da mesma configuracao em duas maquinas
dessincroniza a contagem — nesse caso confira na Sefin e ajuste com
'nfse numero definir'.`,
	}
	cmd.AddCommand(newNumeroVerCommand(), newNumeroDefinirCommand())
	return cmd
}

func newNumeroVerCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "ver",
		Short: "Mostra o ultimo numero usado em cada serie",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}

			statePath := config.StatePath(configPath)
			state, err := config.LoadState(statePath)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(state.Series) == 0 {
				fmt.Fprintf(out, "Nenhuma DPS emitida desta maquina ainda.\n")
				fmt.Fprintf(out, "A proxima da serie %s sera a numero 1.\n", cfg.DPS.Serie)
				return nil
			}

			// Sorted so the output is stable between runs.
			series := make([]string, 0, len(state.Series))
			for s := range state.Series {
				series = append(series, s)
			}
			sort.Strings(series)

			fmt.Fprintf(out, "Ultimo numero usado por serie (registro local):\n\n")
			for _, s := range series {
				info := state.Series[s]
				marca := ""
				if s == cfg.DPS.Serie {
					marca = "  <- serie configurada"
				}
				fmt.Fprintf(out, "  %s  ultimo %d, proximo %d   %s%s\n",
					s, info.LastNumber, info.LastNumber+1,
					info.UpdatedAt.Local().Format("02/01/2006 15:04"), marca)
			}
			fmt.Fprintf(out, "\nArquivo: %s\n", statePath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", config.DefaultFileName, "arquivo de configuracao")
	return cmd
}

func newNumeroDefinirCommand() *cobra.Command {
	var (
		configPath string
		serie      string
	)

	cmd := &cobra.Command{
		Use:   "definir <ultimo-numero>",
		Short: "Ajusta o ultimo numero usado em uma serie",
		Long: `Define manualmente o ultimo numero de DPS usado, para que a proxima emissao
continue a partir dele.

Serve para quem migra de outro emissor e precisa retomar uma numeracao
existente, ou para realinhar a contagem depois de emitir de outra maquina.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			numero, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("%q nao e um numero valido", args[0])
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			alvo := serie
			if alvo == "" {
				alvo = cfg.DPS.Serie
			}

			statePath := config.StatePath(configPath)
			state, err := config.LoadState(statePath)
			if err != nil {
				return err
			}

			anterior := state.LastNumber(alvo)
			state.SetLastNumber(alvo, numero)
			if err := state.Save(statePath); err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Serie %s: ultimo numero %d -> %d\n", alvo, anterior, numero)
			fmt.Fprintf(out, "A proxima emissao usara o numero %d.\n", numero+1)

			// Lowering the counter is legitimate but worth flagging: the next
			// emission will reuse numbers the government may already have.
			if numero < anterior {
				fmt.Fprintf(out, "\nAtencao: a contagem retrocedeu. Se as notas entre %d e %d ja foram\n"+
					"emitidas, a Sefin vai rejeitar a repeticao.\n", numero+1, anterior)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", config.DefaultFileName, "arquivo de configuracao")
	cmd.Flags().StringVar(&serie, "serie", "", "serie a ajustar (padrao: a da configuracao)")
	return cmd
}
