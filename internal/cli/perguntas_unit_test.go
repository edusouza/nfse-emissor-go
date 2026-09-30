package cli

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"testing/iotest"
)

func perguntadorCom(entrada string) (*perguntador, *bytes.Buffer) {
	var out bytes.Buffer
	return &perguntador{in: bufio.NewReader(strings.NewReader(entrada)), out: &out}, &out
}

func TestPerguntar(t *testing.T) {
	casos := []struct {
		nome, entrada, sugestao, quer string
		fim                           bool
	}{
		{nome: "resposta", entrada: "abc\n", quer: "abc"},
		{nome: "ultima linha sem quebra", entrada: "abc", quer: "abc"},
		{nome: "espacos em volta", entrada: "  abc \n", quer: "abc"},
		{nome: "Enter aceita a sugestao", entrada: "\n", sugestao: "00001", quer: "00001"},
		{nome: "Enter sem sugestao", entrada: "\n", quer: ""},
		{nome: "fim da entrada", entrada: "", sugestao: "00001", fim: true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p, out := perguntadorCom(c.entrada)
			got, err := p.perguntar("Pergunta", c.sugestao)
			if c.fim {
				if !errors.Is(err, io.EOF) || got != "" {
					t.Errorf("perguntar = %q, %v; esperava o fim da entrada", got, err)
				}
				return
			}
			if err != nil || got != c.quer {
				t.Errorf("perguntar = %q, %v; esperava %q", got, err, c.quer)
			}
			if c.sugestao != "" && !strings.Contains(out.String(), "["+c.sugestao+"]") {
				t.Errorf("a sugestao nao aparece: %q", out.String())
			}
		})
	}
}

// An empty answer means "leave it blank": it is not checked, and not asked
// again.
func TestAteValerRespostaVazia(t *testing.T) {
	p, out := perguntadorCom("\n")
	chamadas := 0
	got, err := p.ateValer("Pergunta", "", func(string) error { chamadas++; return nil })
	if got != "" || err != nil || chamadas != 0 || strings.Count(out.String(), "Pergunta") != 1 {
		t.Errorf("ateValer = %q, %v; %d verificacoes, saida %q", got, err, chamadas, out.String())
	}
}

// A read error other than the end of input ends the questions too, and the
// file is still written with what was discovered.
func TestOnboardInterativoErroDeLeitura(t *testing.T) {
	comoTerminal(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	var out bytes.Buffer
	path := t.TempDir() + "/nfse.yaml"
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(iotest.ErrReader(errors.New("terminal fechado")))
	root.SetArgs([]string{"onboard", "--certificado", certificadoDeTeste(t), "--senha", onboardPassword,
		"--arquivo", path, "--sem-rede", "--cache-municipios", t.TempDir() + "/m.json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "falha ao ler a resposta (terminal fechado)") ||
		!strings.Contains(out.String(), "nfse.yaml criado") {
		t.Errorf("esperava o aviso e o arquivo:\n%s", out.String())
	}
}

// The field lengths are the XSD's, counted in characters: accents do not
// make a name that fits look too long.
func TestTextoAte(t *testing.T) {
	casos := []struct {
		nome, texto string
		ok          bool
	}{
		{"cabe", strings.Repeat("ç", 300), true},
		{"passa do limite", strings.Repeat("a", 301), false},
		{"tab vira espaco no arquivo", "Consultoria\tmensal", true},
		{"seta", "Consultoria\x1b[D", false},
		{"nao-caractere que o XML recusa", "Consultoria\ufffe", false},
		{"bytes que nao sao UTF-8", "Consultoria \xe7", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if err := textoAte(300)(c.texto); (err == nil) != c.ok {
				t.Errorf("textoAte(300) = %v; esperava ok=%v", err, c.ok)
			}
		})
	}
}

// Whether to ask depends on the reader the answers come from, not on
// os.Stdin: a command handed a buffer never waits on the terminal.
func TestEntradaEhTerminal(t *testing.T) {
	if entradaEhTerminal(strings.NewReader("mei\n")) {
		t.Error("um buffer foi tomado por terminal")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if entradaEhTerminal(r) {
		t.Error("um pipe foi tomado por terminal")
	}
}
