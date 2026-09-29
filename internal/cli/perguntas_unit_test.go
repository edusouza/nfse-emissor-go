package cli

import (
	"bufio"
	"bytes"
	"errors"
	"io"
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
