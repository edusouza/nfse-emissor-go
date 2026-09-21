package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edusouza/nfse-emissor-go/internal/domain/municipio"
)

func newMunicipioCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "municipio",
		Short: "Consulta o codigo IBGE dos municipios",
		Long: `Procura os 7 digitos do codigo IBGE que vao na configuracao.

O codigo do municipio do prestador e o campo mais chato de preencher a mao, e
um erro nele nao aparece localmente: a nota e montada, assinada, enviada, e vai
para o municipio errado. A tabela inteira — 5570 municipios — esta embutida no
binario, entao a consulta e local e funciona sem rede.`,
	}
	cmd.AddCommand(newMunicipioBuscarCommand(), newMunicipioVerCommand())
	return cmd
}

func newMunicipioBuscarCommand() *cobra.Command {
	var limite int

	cmd := &cobra.Command{
		Use:   "buscar <nome>",
		Short: "Procura municipios pelo nome",
		Long: `Procura na tabela do IBGE pelo nome do municipio.

O nome pode vir sem acento e em qualquer caixa. Acrescente a UF para restringir
a um estado.

Exemplos:
  nfse municipio buscar curitiba
  nfse municipio buscar "bom jesus/SC"
  nfse municipio buscar "sao paulo"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			consulta := strings.Join(args, " ")
			out := cmd.OutOrStdout()

			resultados := municipio.Buscar(consulta, limite)
			if len(resultados) == 0 {
				fmt.Fprintf(out, "Nenhum municipio casa com %q.\n\n", consulta)
				fmt.Fprintf(out, "A tabela usa o nome oficial do IBGE, com acento e por extenso.\n")
				return nil
			}

			fmt.Fprintf(out, "%s para %q:\n\n", plural(len(resultados), "resultado", "resultados"), consulta)
			for _, m := range resultados {
				fmt.Fprintf(out, "  %s  %s\n", m.Codigo, m)
			}
			fmt.Fprintf(out, "\nPara usar um deles:\n")
			fmt.Fprintf(out, "  nfse onboard --certificado seu.pfx --municipio %q\n", resultados[0].String())
			return nil
		},
	}

	cmd.Flags().IntVarP(&limite, "limite", "n", 15, "quantos resultados mostrar (0 mostra todos)")
	return cmd
}

func newMunicipioVerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ver <codigo|nome>",
		Short: "Mostra o municipio de um codigo IBGE, ou o codigo de um nome",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := municipio.Resolver(strings.Join(args, " "))
			if err != nil {
				return erroDeMunicipio(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", m.Codigo, m)
			return nil
		},
	}
}

// erroDeMunicipio turns the domain's typed errors into something the user can
// act on. The domain carries the candidates; saying what to do with them is
// this layer's job.
func erroDeMunicipio(err error) error {
	var ambiguo *municipio.ErroAmbiguo
	if errors.As(err, &ambiguo) {
		var sb strings.Builder
		fmt.Fprintf(&sb, "%q existe em mais de um estado; acrescente a UF:\n", ambiguo.Consulta)
		for _, c := range ambiguo.Candidatos {
			fmt.Fprintf(&sb, "  %s  %s\n", c.Codigo, c)
		}
		return errors.New(strings.TrimRight(sb.String(), "\n"))
	}

	var naoEncontrado *municipio.ErroNaoEncontrado
	if errors.As(err, &naoEncontrado) {
		var sb strings.Builder
		fmt.Fprintf(&sb, "%q nao esta na tabela do IBGE (%s)", naoEncontrado.Consulta, municipio.Anexo)
		if len(naoEncontrado.Sugestoes) > 0 {
			sb.WriteString("\nvoce quis dizer:\n")
			for _, s := range naoEncontrado.Sugestoes {
				fmt.Fprintf(&sb, "  %s  %s\n", s.Codigo, s)
			}
			return errors.New(strings.TrimRight(sb.String(), "\n"))
		}
		sb.WriteString(";\nprocure pelo nome com 'nfse municipio buscar <nome>'")
		return errors.New(sb.String())
	}

	return err
}

// avisarDivergenciaDeMunicipio reports a --municipio that disagrees with the
// public registry.
//
// The flag wins: it is what the user typed, now, on purpose. But the registry
// knows the address the Receita has on file, and a disagreement usually means
// one of the two is stale — worth saying out loud rather than resolving in
// silence.
func avisarDivergenciaDeMunicipio(errOut io.Writer, informado, doCadastro string) {
	daTabela := func(codigo string) string {
		if m, ok := municipio.PorCodigo(codigo); ok {
			return fmt.Sprintf("%s (%s)", codigo, m)
		}
		return codigo
	}

	fmt.Fprintf(errOut, "aviso: --municipio informa %s, mas o cadastro publico diz %s.\n",
		daTabela(informado), daTabela(doCadastro))
	fmt.Fprintf(errOut, "aviso: vale o que voce informou; confira qual dos dois esta certo\n")
}
