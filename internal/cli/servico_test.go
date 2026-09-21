// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// execNfse runs the command tree and returns stdout and stderr merged, the way
// a terminal shows them.
func execNfse(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)

	err := root.Execute()
	return out.String(), err
}

func TestServicoBuscar(t *testing.T) {
	out, err := execNfse(t, "servico", "buscar", "suporte", "tecnico")
	if err != nil {
		t.Fatalf("busca falhou: %v\n%s", err, out)
	}
	if !strings.Contains(out, "010701") {
		t.Errorf("a busca nao trouxe 010701:\n%s", out)
	}
	if !strings.Contains(out, "subitem 1.07") {
		t.Errorf("a saida nao situa o codigo na lei:\n%s", out)
	}
}

// A code pasted where words were expected is a lookup, and answering it saves
// the user from being told there are no results for their own code.
func TestServicoBuscarAceitaCodigo(t *testing.T) {
	out, err := execNfse(t, "servico", "buscar", "010701")
	if err != nil {
		t.Fatalf("busca falhou: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Suporte") {
		t.Errorf("um codigo nao foi reconhecido como codigo:\n%s", out)
	}
}

// The list speaks the language of the law, which is not always the language of
// the trade. An empty result has to say so and offer a way out.
func TestServicoBuscarSemResultadoOrienta(t *testing.T) {
	out, err := execNfse(t, "servico", "buscar", "xyzzy")
	if err != nil {
		t.Fatalf("uma busca sem resultado nao e um erro: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Nenhum codigo casa") {
		t.Errorf("a saida nao diz que nao achou:\n%s", out)
	}
	if !strings.Contains(out, "nfse servico listar") {
		t.Errorf("a saida nao oferece outro caminho:\n%s", out)
	}
}

func TestServicoVer(t *testing.T) {
	out, err := execNfse(t, "servico", "ver", "171901")
	if err != nil {
		t.Fatalf("ver falhou: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Contabilidade") {
		t.Errorf("a descricao nao apareceu:\n%s", out)
	}

	out, err = execNfse(t, "servico", "ver", "999999")
	if err == nil {
		t.Fatalf("um codigo inexistente deveria ser recusado\n%s", out)
	}
	if !strings.Contains(err.Error(), "nfse servico buscar") {
		t.Errorf("o erro nao diz o que fazer: %v", err)
	}
}

func TestServicoListar(t *testing.T) {
	out, err := execNfse(t, "servico", "listar")
	if err != nil {
		t.Fatalf("listar falhou: %v\n%s", err, out)
	}
	if !strings.Contains(out, "41 itens") {
		t.Errorf("a lista de itens nao saiu inteira:\n%s", out)
	}

	out, err = execNfse(t, "servico", "listar", "1")
	if err != nil {
		t.Fatalf("listar item falhou: %v\n%s", err, out)
	}
	for _, want := range []string{"010101", "010701", "Serviços de Informática"} {
		if !strings.Contains(out, want) {
			t.Errorf("faltou %q no item 1:\n%s", want, out)
		}
	}

	if _, err := execNfse(t, "servico", "listar", "77"); err == nil {
		t.Error("um item inexistente deveria ser recusado")
	}
}

// config check is where a hand-typed code gets a second reading: it says what
// the six digits mean, and warns when this binary has never heard of them.
func TestConfigCheckDescreveOServico(t *testing.T) {
	out, err := execNfse(t, "config", "check", "--arquivo", escreverConfig(t, "010701"))
	if err != nil {
		t.Fatalf("config check falhou: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Servico     010701") || !strings.Contains(out, "Suporte") {
		t.Errorf("config check nao diz o que o codigo significa:\n%s", out)
	}
}

func TestConfigCheckAvisaServicoDesconhecido(t *testing.T) {
	// 999999 is well formed — six digits — and passes every local rule the
	// emitter has. Only the list can say it does not exist.
	out, err := execNfse(t, "config", "check", "--arquivo", escreverConfig(t, "999999"))
	if err != nil {
		t.Fatalf("um codigo desconhecido e aviso, nao erro: %v\n%s", err, out)
	}
	if !strings.Contains(out, "aviso: 999999 nao esta na lista nacional") {
		t.Errorf("config check nao avisou:\n%s", out)
	}
}

func escreverConfig(t *testing.T, codigo string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "nfse.yaml")
	conteudo := `ambiente: producao-restrita
certificado:
  arquivo: ./certificado.pfx
prestador:
  cnpj: "12345678000195"
  nome: EMPRESA TESTE LTDA
  regime_tributario: mei
  municipio: "4106902"
dps:
  serie: "00001"
padroes:
  servico:
    codigo_tributacao_nacional: "` + codigo + `"
    descricao: Servico de teste
saida:
  diretorio: ./notas
`
	if err := os.WriteFile(path, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
