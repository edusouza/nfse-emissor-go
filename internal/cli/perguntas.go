package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/edusouza/nfse-emissor-go/internal/config"
	"github.com/edusouza/nfse-emissor-go/internal/domain/servico"
)

// stdinEhTerminal decides whether onboard may ask questions: the same test the
// password prompt uses. A variable so that tests can stand in for a terminal.
var stdinEhTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// tentativas bounds how often one question is asked again after an invalid
// answer. Past it the field is left blank and listed as pending: a file with
// a gap beats a prompt that never lets go.
const tentativas = 3

// perguntador reads answers from the terminal, one line each.
type perguntador struct {
	in  *bufio.Reader
	out io.Writer
}

// perguntar shows the question, with the suggested value in brackets when
// there is one, and returns the answer — or the suggestion, for an empty
// line. io.EOF means the user closed the input: stop asking.
func (p *perguntador) perguntar(pergunta, sugestao string) (string, error) {
	if sugestao != "" {
		fmt.Fprintf(p.out, "%s [%s]: ", pergunta, sugestao)
	} else {
		fmt.Fprintf(p.out, "%s: ", pergunta)
	}

	linha, err := p.in.ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || linha == "") {
		fmt.Fprintln(p.out)
		return "", err
	}
	if resposta := strings.TrimSpace(linha); resposta != "" {
		return resposta, nil
	}
	return sugestao, nil
}

// ateValer asks until aceitar takes the answer, an empty answer is given, or
// the attempts run out. aceitar returns what to say about a refused answer.
func (p *perguntador) ateValer(pergunta, sugestao string, aceitar func(string) error) (string, error) {
	for i := 0; i < tentativas; i++ {
		resposta, err := p.perguntar(pergunta, sugestao)
		if err != nil || resposta == "" {
			return "", err
		}
		problema := aceitar(resposta)
		if problema == nil {
			return resposta, nil
		}
		fmt.Fprintf(p.out, "  %v\n", problema)
	}
	if sugestao != "" {
		fmt.Fprintf(p.out, "  Fica o valor sugerido, %s.\n", sugestao)
		return sugestao, nil
	}
	fmt.Fprintf(p.out, "  O campo fica em branco; preencha depois no arquivo.\n")
	return "", nil
}

// textoImprimivel refuses what a terminal can slip into a line: an arrow key
// pressed to fix a typo arrives as ESC [ D, a Latin-1 terminal sends bytes
// that are not UTF-8. Either would make nfse.yaml unreadable, and C0 control
// characters are not even allowed in the XML the text ends up in.
func textoImprimivel(v string) error {
	if !utf8.ValidString(v) {
		return errors.New("o texto tem bytes que nao sao UTF-8; confira a codificacao do terminal e digite de novo")
	}
	for _, r := range v {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			return fmt.Errorf("o texto tem um caractere de controle (%U), talvez de uma seta ou de um atalho; digite de novo sem ele", r)
		}
	}
	return nil
}

const digitadoNoTerminal = "digitado no terminal"

// completarNoTerminal asks for what discovery left blank, and only that: a
// field the certificate or the registry answered is not asked again.
//
// The tax regime gets no suggestion. The registry either answered it or
// said it did not know, and suggesting one after "do not know" would be
// guessing a field that decides regApTribSN on every invoice (ADR 0007). The
// service code gets none either: the candidates are a word ranking, not an
// answer (ADR 0009).
func completarNoTerminal(p *perguntador, data *onboardData, busca buscaMunicipio, serie *string, perguntarSerie bool) {
	busca.origem = digitadoNoTerminal
	fmt.Fprintf(p.out, "\nFaltam alguns campos. Enter deixa o campo em branco, para preencher depois no arquivo.\n\n")

	passos := []func() error{
		func() error {
			if data.nome != "" {
				return nil
			}
			nome, err := p.ateValer("Razao social", "", textoImprimivel)
			if nome != "" {
				data.nome = nome
				data.origem("prestador.nome", digitadoNoTerminal)
			}
			return err
		},
		func() error {
			if data.municipio != "" {
				return nil
			}
			var achado municipioInformado
			_, err := p.ateValer("Municipio (codigo IBGE ou Cidade/UF)", "", func(v string) error {
				m, err := busca.resolver(v)
				if err != nil {
					return err
				}
				achado = m
				return nil
			})
			if achado.codigo != "" {
				data.municipio = achado.codigo
				data.origem("prestador.municipio", achado.fonte)
				fmt.Fprintf(p.out, "  %s\n", achado)
			}
			return err
		},
		func() error {
			if data.regime != "" {
				return nil
			}
			fmt.Fprintf(p.out, "Regime tributario: mei e o Microempreendedor Individual; me_epp, a ME ou EPP do Simples Nacional.\n")
			regime, err := p.ateValer("Regime (mei | me_epp)", "", func(v string) error {
				if v != config.RegimeMEI && v != config.RegimeMEEPP {
					return fmt.Errorf("%q nao e um regime; responda mei ou me_epp", v)
				}
				return nil
			})
			if regime != "" {
				data.regime = regime
				data.origem("prestador.regime_tributario", digitadoNoTerminal)
			}
			return err
		},
		func() error {
			if !perguntarSerie {
				return nil
			}
			s, err := p.ateValer("Serie da DPS", *serie, func(v string) error {
				if !serieValida(v) {
					return fmt.Errorf("%q nao tem 5 digitos", v)
				}
				return nil
			})
			if s != "" {
				*serie = s
			}
			return err
		},
		func() error {
			if data.servico.Codigo != "" {
				return nil
			}
			var escolhido servico.Servico
			_, err := p.ateValer("Codigo do servico (cTribNac, 6 digitos)", "", func(v string) error {
				s, ok := servico.PorCodigo(v)
				if !ok {
					return fmt.Errorf("%q nao esta na lista nacional; procure com 'nfse servico buscar <termo>'", v)
				}
				escolhido = s
				return nil
			})
			if escolhido.Codigo != "" {
				data.servico = escolhido
				data.origem("padroes.servico.codigo_tributacao_nacional", digitadoNoTerminal)
				fmt.Fprintf(p.out, "  %s\n", umaLinha(escolhido.Descricao, larguraTexto-2))
			}
			return err
		},
		func() error {
			descricao, err := p.ateValer("Descricao padrao do servico, a que vai na nota", "", textoImprimivel)
			if descricao != "" {
				data.descricao = descricao
				data.origem("padroes.servico.descricao", digitadoNoTerminal)
			}
			return err
		},
	}

	for _, passo := range passos {
		if err := passo(); err != nil {
			// Ctrl-D is the user done answering. Any other read error ends
			// the questions the same way: what the certificate and the
			// registry answered is still worth a file.
			if !errors.Is(err, io.EOF) {
				fmt.Fprintf(p.out, "aviso: falha ao ler a resposta (%v); o arquivo sai com o que ja se sabe\n", err)
			}
			return
		}
	}
}
