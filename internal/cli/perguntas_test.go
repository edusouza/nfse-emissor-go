package cli

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// comoTerminal makes onboard believe stdin is a terminal for one test.
func comoTerminal(t *testing.T) {
	t.Helper()
	anterior := stdinEhTerminal
	stdinEhTerminal = func() bool { return true }
	t.Cleanup(func() { stdinEhTerminal = anterior })
}

// onboardRespondendo runs onboard with the given lines typed at the prompt.
func onboardRespondendo(t *testing.T, respostas string, args ...string) (string, string, error) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	path := filepath.Join(t.TempDir(), "nfse.yaml")
	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(respostas))
	root.SetArgs(append([]string{"onboard",
		"--certificado", certificadoDeTeste(t), "--senha", onboardPassword,
		"--arquivo", path, "--cache-municipios", filepath.Join(t.TempDir(), "municipios.json"),
	}, args...))

	err := root.Execute()
	gerado, _ := os.ReadFile(path)
	return out.String(), string(gerado), err
}

// Offline, the certificate gives CNPJ and name; everything else is asked, a
// wrong answer is asked again, and Enter takes the suggested series.
func TestOnboardInterativoCompletaOQueFalta(t *testing.T) {
	comoTerminal(t)

	respostas := strings.Join([]string{
		"4106902",     // municipio
		"simples",     // regime invalido
		"mei",         // regime
		"",            // serie: aceita a sugestao
		"010101",      // cTribNac
		"Consultoria", // descricao
	}, "\n") + "\n"

	out, gerado, err := onboardRespondendo(t, respostas, "--sem-rede")
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}

	for _, want := range []string{
		`municipio: "4106902"`, `regime_tributario: "mei"`, `serie: "00001"`,
		`codigo_tributacao_nacional: "010101"`, `descricao: "Consultoria"`,
		"prestador.regime_tributario: digitado no terminal",
	} {
		if !strings.Contains(gerado, want) {
			t.Errorf("o arquivo nao traz %q:\n%s", want, gerado)
		}
	}
	for _, want := range []string{"Serie da DPS [00001]:", `"simples" nao e um regime`, "Nada ficou em branco"} {
		if !strings.Contains(out, want) {
			t.Errorf("a saida nao traz %q:\n%s", want, out)
		}
	}
	// The name came from the certificate, so it is not asked.
	if strings.Contains(out, "Razao social:") {
		t.Errorf("perguntou o que o certificado ja respondeu:\n%s", out)
	}
	// The regime is explained, never suggested.
	if strings.Contains(out, "Regime (mei | me_epp) [") {
		t.Errorf("sugeriu um regime:\n%s", out)
	}
}

// What the registry answered is not asked again.
func TestOnboardInterativoNaoRepeteOCadastro(t *testing.T) {
	comoTerminal(t)
	registro := registroFake(t, http.StatusOK, respostaMEI)

	out, gerado, err := onboardRespondendo(t, "\n\n\n\n", "--fonte", registro.URL)
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	for _, pergunta := range []string{"Municipio (", "Regime (", "Razao social:"} {
		if strings.Contains(out, pergunta) {
			t.Errorf("perguntou %q, que o cadastro ja respondeu:\n%s", pergunta, out)
		}
	}
	if !strings.Contains(gerado, `municipio: "3550308"`) {
		t.Errorf("o cadastro nao foi aproveitado:\n%s", gerado)
	}
}

// Ctrl-D ends the questions, not the command: the file is written with what
// is known and the rest is listed.
func TestOnboardInterativoFimDaEntrada(t *testing.T) {
	comoTerminal(t)

	out, gerado, err := onboardRespondendo(t, "", "--sem-rede")
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if gerado == "" {
		t.Fatal("o arquivo nao foi gravado")
	}
	if !strings.Contains(out, "prestador.municipio —") {
		t.Errorf("o que faltou nao foi listado:\n%s", out)
	}
}

// Three wrong answers leave the field blank instead of holding the prompt.
func TestOnboardInterativoDesisteDepoisDeTresTentativas(t *testing.T) {
	comoTerminal(t)

	out, gerado, err := onboardRespondendo(t, "4106903\n7535\nBom Jesus\n", "--sem-rede")
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if strings.Count(out, "Municipio (codigo IBGE ou Cidade/UF):") != tentativas {
		t.Errorf("esperava %d perguntas pelo municipio:\n%s", tentativas, out)
	}
	if !strings.Contains(out, "O campo fica em branco") || municipioGravado.MatchString(gerado) {
		t.Errorf("o municipio deveria ter ficado em branco:\n%s", gerado)
	}
}

func TestOnboardNaoInterativo(t *testing.T) {
	comoTerminal(t)

	out, _, err := onboardRespondendo(t, "4106902\nmei\n", "--sem-rede", "--nao-interativo")
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if strings.Contains(out, "Faltam alguns campos") {
		t.Errorf("perguntou mesmo com --nao-interativo:\n%s", out)
	}
}

// Without a terminal — a script, a pipe — nothing is asked, whatever stdin
// holds.
func TestOnboardSemTerminalNaoPergunta(t *testing.T) {
	out, _, err := onboardRespondendo(t, "4106902\nmei\n", "--sem-rede")
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if strings.Contains(out, "Faltam alguns campos") {
		t.Errorf("perguntou sem terminal:\n%s", out)
	}
}

// An explicit --serie is an answer already.
func TestOnboardInterativoRespeitaASerieInformada(t *testing.T) {
	comoTerminal(t)

	out, gerado, err := onboardRespondendo(t, "\n\n\n\n", "--sem-rede", "--serie", "00042")
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if strings.Contains(out, "Serie da DPS") || !strings.Contains(gerado, `serie: "00042"`) {
		t.Errorf("perguntou a serie informada, ou nao a gravou:\n%s\n%s", out, gerado)
	}
}
