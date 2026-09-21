// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package servico

import "testing"

// TestBuscarEncontraOCodigoEsperado fixes the answers a person would check the
// search against. They are not the whole ranking — only that the right code
// comes back, and comes back near the top.
func TestBuscarEncontraOCodigoEsperado(t *testing.T) {
	casos := []struct {
		consulta string
		codigo   string
		dentroDe int
	}{
		{"suporte tecnico em informatica", "010701", 1},
		{"consultoria em informatica", "010601", 1},
		{"analise e desenvolvimento de sistemas", "010101", 1},
		{"advocacia", "171401", 1},
		{"contabilidade", "171901", 1},
		// Without accents, as a terminal invites.
		{"veterinario", "050101", 3},
		// Plural, which the search handles by prefix rather than by a
		// stemmer, and a word the list only carries in the singular.
		{"treinamentos", "080201", 2},
		{"transporte municipal de passageiros", "160101", 1},
	}

	for _, caso := range casos {
		resultados := Buscar(caso.consulta, 10)
		posicao := -1
		for i, r := range resultados {
			if r.Servico.Codigo == caso.codigo {
				posicao = i + 1
				break
			}
		}
		switch {
		case posicao < 0:
			t.Errorf("%q nao trouxe %s (veio %s)", caso.consulta, caso.codigo, primeiros(resultados))
		case posicao > caso.dentroDe:
			t.Errorf("%q trouxe %s na posicao %d, esperava ate %d (%s)",
				caso.consulta, caso.codigo, posicao, caso.dentroDe, primeiros(resultados))
		}
	}
}

func TestBuscarNaoInventaResposta(t *testing.T) {
	for _, consulta := range []string{"", "   ", "xyzzy", "de da do", "a"} {
		if got := Buscar(consulta, 5); len(got) != 0 {
			t.Errorf("%q devolveu %d resultados; uma busca sem casamento tem que voltar vazia: %s",
				consulta, len(got), primeiros(got))
		}
	}
}

func TestBuscarRespeitaOLimite(t *testing.T) {
	if got := Buscar("servicos", 3); len(got) > 3 {
		t.Errorf("limite 3 devolveu %d resultados", len(got))
	}
	if got := Buscar("transporte", 0); len(got) == 0 {
		t.Error("limite 0 deveria significar sem limite")
	}
}

// TestSugerirPorCNAE exercises the case the onboarding was built for: the CNAE
// text that the public registry answers for a software provider.
//
// The assertion is deliberately loose — "among the first three" — because the
// suggestion is a ranking of candidates, not a mapping. The same CNAE text
// contains "tecnologia da informação", which appears verbatim inside a code
// about vehicle tracking; the right answer has to beat it, but the list is
// there for a person to read.
func TestSugerirPorCNAE(t *testing.T) {
	const cnae6209100 = "Suporte técnico, manutenção e outros serviços em tecnologia da informação"

	resultados := SugerirPorCNAE(cnae6209100, 3)
	if len(resultados) == 0 {
		t.Fatal("nenhuma sugestao para o CNAE 6209-1/00")
	}
	if resultados[0].Servico.Codigo != "010701" {
		t.Errorf("a primeira sugestao foi %s, esperava 010701 (%s)",
			resultados[0].Servico.Codigo, primeiros(resultados))
	}
}

func TestSugerirPorCNAESemDescricao(t *testing.T) {
	if got := SugerirPorCNAE("", 5); len(got) != 0 {
		t.Errorf("um CNAE sem descricao nao pode gerar sugestao, veio %s", primeiros(got))
	}
}

func TestNormalizarDobraAcentosEDescartaRuido(t *testing.T) {
	got := normalizar("Manutenção, conservação e congêneres (Lei nº 12.485/2011).")

	// Bare numbers and words under three letters are dropped: a law number
	// separates nothing, and "e" matches everything. What is left is the
	// accent-free lowercase words.
	want := []string{"manutencao", "conservacao", "congeneres", "lei"}
	if len(got) != len(want) {
		t.Fatalf("normalizar devolveu %q, esperava %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizar devolveu %q, esperava %q", got, want)
		}
	}
}

func primeiros(resultados []Resultado) string {
	out := ""
	for i, r := range resultados {
		if i == 3 {
			break
		}
		if i > 0 {
			out += ", "
		}
		out += r.Servico.Codigo
	}
	if out == "" {
		return "(vazio)"
	}
	return out
}
