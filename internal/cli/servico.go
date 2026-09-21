package cli

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/edusouza/nfse-emissor-go/internal/domain/servico"
)

// larguraTexto is where descriptions wrap. The list has entries of 450
// characters; on one line they are unreadable.
const larguraTexto = 76

func newServicoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "servico",
		Short: "Consulta a lista nacional de servicos (cTribNac)",
		Long: `Procura o codigo de tributacao nacional que vai na nota.

O cTribNac tem 6 digitos e sai da lista de servicos da LC 116/2003, publicada
pelo Sistema Nacional NFS-e. A lista inteira — 335 codigos — esta embutida no
binario, entao a consulta e local e funciona sem rede.

Nenhum servico publico responde qual e o seu codigo: ele depende do que voce
presta, nao de quem voce e. Estes comandos ajudam a achar; a escolha e sua.`,
	}
	cmd.AddCommand(newServicoBuscarCommand(), newServicoVerCommand(), newServicoListarCommand())
	return cmd
}

func newServicoBuscarCommand() *cobra.Command {
	var limite int

	cmd := &cobra.Command{
		Use:   "buscar <termo>...",
		Short: "Procura codigos pela descricao do servico",
		Long: `Procura na lista nacional pelas palavras do servico prestado.

Os termos podem vir sem acento e no plural. A busca pesa mais as palavras raras
na lista: "servicos" e "congeneres" aparecem em quase todo codigo e nao
separam nada, enquanto "veterinario" ou "dragagem" apontam para um punhado.

Exemplos:
  nfse servico buscar suporte tecnico
  nfse servico buscar "aulas de idiomas"
  nfse servico buscar advocacia --limite 3`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			consulta := strings.Join(args, " ")
			out := cmd.OutOrStdout()

			// A code pasted in where words were expected is a lookup, not a
			// search, and answering it as one saves the user a second command.
			if s, ok := servico.PorCodigo(consulta); ok {
				imprimirServico(out, s)
				return nil
			}

			resultados := servico.Buscar(consulta, limite)
			if len(resultados) == 0 {
				fmt.Fprintf(out, "Nenhum codigo casa com %q.\n\n", consulta)
				fmt.Fprintf(out, "A lista usa os termos da lei, que nem sempre sao os do dia a dia:\n")
				fmt.Fprintf(out, "  \"aula\" esta como \"ensino\"; \"software\" como \"programa de computacao\".\n")
				fmt.Fprintf(out, "Tente outra palavra, ou percorra os grupos:\n  nfse servico listar\n")
				return nil
			}

			fmt.Fprintf(out, "%s para %q:\n\n", plural(len(resultados), "resultado", "resultados"), consulta)
			for _, r := range resultados {
				imprimirServico(out, r.Servico)
			}
			fmt.Fprintf(out, "Para usar um deles:\n")
			fmt.Fprintf(out, "  nfse onboard --certificado seu.pfx --servico %s\n", resultados[0].Servico.Codigo)
			fmt.Fprintf(out, "ou escreva o codigo em padroes.servico.codigo_tributacao_nacional.\n")
			return nil
		},
	}

	cmd.Flags().IntVarP(&limite, "limite", "n", 10, "quantos resultados mostrar (0 mostra todos)")
	return cmd
}

func newServicoVerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ver <codigo>",
		Short: "Mostra o servico de um codigo de tributacao nacional",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, ok := servico.PorCodigo(args[0])
			if !ok {
				return fmt.Errorf("%q nao esta na lista nacional (%s);\n"+
					"procure pela descricao com 'nfse servico buscar'",
					args[0], servico.Anexo)
			}
			imprimirServico(cmd.OutOrStdout(), s)
			return nil
		},
	}
}

func newServicoListarCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "listar [item]",
		Short: "Lista os grupos da lei, ou os codigos de um grupo",
		Long: `Sem argumento, lista os 41 itens da LC 116/2003.

Com o numero de um item, lista os codigos dele. E o caminho para quem nao
acerta a palavra que a lei usa: ache o grupo, depois o codigo.

  nfse servico listar
  nfse servico listar 1`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			if len(args) == 0 {
				grupos := servico.Grupos()
				fmt.Fprintf(out, "%d itens da LC 116/2003:\n\n", len(grupos))
				for _, g := range grupos {
					fmt.Fprintf(out, "  %2s  %s\n", g.Item, umaLinha(g.Nome, larguraTexto))
				}
				fmt.Fprintf(out, "\nPara ver os codigos de um item:\n  nfse servico listar 1\n")
				return nil
			}

			servicos := servico.PorItem(args[0])
			if len(servicos) == 0 {
				return fmt.Errorf("nao existe o item %q na lista nacional;\n"+
					"rode 'nfse servico listar' para ver os itens", args[0])
			}

			fmt.Fprintf(out, "Item %s — %s\n\n", servicos[0].Item(), servicos[0].Grupo)
			for _, s := range servicos {
				fmt.Fprintf(out, "  %s  %s\n", s.Codigo, s.Subitem())
				for _, linha := range quebrar(s.Descricao, larguraTexto-12) {
					fmt.Fprintf(out, "            %s\n", linha)
				}
			}
			return nil
		},
	}
}

// imprimirServico renders one entry: the code first, because it is what gets
// copied, then the description, then the item it belongs to.
func imprimirServico(out io.Writer, s servico.Servico) {
	fmt.Fprintf(out, "  %s  (subitem %s da LC 116/2003)\n", s.Codigo, s.Subitem())
	for _, linha := range quebrar(s.Descricao, larguraTexto-4) {
		fmt.Fprintf(out, "    %s\n", linha)
	}
	fmt.Fprintf(out, "    item %s — %s\n\n", s.Item(), umaLinha(s.Grupo, larguraTexto-11))
}

// quebrar wraps text at word boundaries.
//
// Widths are counted in characters, not bytes: the list is in Portuguese, and
// measuring "instalação" as thirteen would wrap every line short.
func quebrar(texto string, largura int) []string {
	if largura < 20 {
		largura = 20
	}

	var linhas []string
	var atual strings.Builder
	usado := 0

	for _, palavra := range strings.Fields(texto) {
		n := utf8.RuneCountInString(palavra)
		switch {
		case usado == 0:
			atual.WriteString(palavra)
			usado = n
		case usado+1+n <= largura:
			atual.WriteString(" ")
			atual.WriteString(palavra)
			usado += 1 + n
		default:
			linhas = append(linhas, atual.String())
			atual.Reset()
			atual.WriteString(palavra)
			usado = n
		}
	}
	if usado > 0 {
		linhas = append(linhas, atual.String())
	}
	return linhas
}

// umaLinha truncates instead of wrapping, for places where the text is a label
// rather than content. It cuts at a word boundary, and never inside a
// character.
func umaLinha(texto string, largura int) string {
	if utf8.RuneCountInString(texto) <= largura {
		return texto
	}

	runas := []rune(texto)
	corte := largura - 3
	for i := corte; i > largura/2; i-- {
		if runas[i] == ' ' {
			corte = i
			break
		}
	}
	return strings.TrimRight(string(runas[:corte]), " ,.;") + "..."
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
