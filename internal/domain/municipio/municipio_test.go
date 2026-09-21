package municipio

import (
	"errors"
	"strconv"
	"testing"
)

func TestTabelaTemOsMunicipiosDoAnexo(t *testing.T) {
	todos := Todos()

	// The IBGE table has 5570 municipalities. Asserting the count catches a
	// generator that silently dropped rows.
	if len(todos) != 5570 {
		t.Fatalf("tabela tem %d municipios, esperava 5570", len(todos))
	}

	vistos := make(map[string]bool, len(todos))
	anterior := ""
	for _, m := range todos {
		if len(m.Codigo) != 7 {
			t.Errorf("codigo %q nao tem 7 digitos", m.Codigo)
		}
		if _, err := strconv.Atoi(m.Codigo); err != nil {
			t.Errorf("codigo %q nao e numerico", m.Codigo)
		}
		if m.Nome == "" {
			t.Errorf("codigo %s sem nome", m.Codigo)
		}
		if len(m.UF) != 2 {
			t.Errorf("codigo %s tem UF %q", m.Codigo, m.UF)
		}
		if vistos[m.Codigo] {
			t.Errorf("codigo %s aparece duas vezes", m.Codigo)
		}
		vistos[m.Codigo] = true

		if m.Codigo <= anterior {
			t.Errorf("codigo %s vem depois de %s; a tabela deveria estar ordenada", m.Codigo, anterior)
		}
		anterior = m.Codigo
	}
}

// TestChaveNaoFundeMunicipios guards the decision in chave(): punctuation is
// dropped entirely rather than normalised, which is only safe while no two
// municipalities of one state differ solely by it. If a future annex adds such
// a pair, this fails and the folding has to change — silently fusing two
// cities would send notes to the wrong one.
func TestChaveNaoFundeMunicipios(t *testing.T) {
	vistos := map[string]Municipio{}
	for _, m := range Todos() {
		k := chave(m.Nome) + "/" + m.UF
		if anterior, ok := vistos[k]; ok {
			t.Errorf("%q (%s) e %q (%s) viram a mesma chave %q",
				anterior.Nome, anterior.Codigo, m.Nome, m.Codigo, k)
		}
		vistos[k] = m
	}
}

func TestPorCodigo(t *testing.T) {
	casos := map[string]string{
		"4106902":   "Curitiba/PR",
		"3550308":   "São Paulo/SP",
		"5300108":   "Brasília/DF",
		"41 069 02": "Curitiba/PR",
	}
	for entrada, querido := range casos {
		m, ok := PorCodigo(entrada)
		if !ok {
			t.Fatalf("%q nao foi encontrado", entrada)
		}
		if m.String() != querido {
			t.Errorf("%q devolveu %s, esperava %s", entrada, m, querido)
		}
	}

	for _, entrada := range []string{"", "9999999", "410690"} {
		if _, ok := PorCodigo(entrada); ok {
			t.Errorf("%q nao deveria ser encontrado", entrada)
		}
	}
}

// TestResolverAceitaComoAsPessoasEscrevem covers the shapes a terminal
// invites: with and without the state, without accents, in any case, and the
// code itself pasted back out of a previous nfse.yaml.
func TestResolverAceitaComoAsPessoasEscrevem(t *testing.T) {
	casos := map[string]string{
		"Curitiba/PR":           "4106902",
		"curitiba/pr":           "4106902",
		"Curitiba - PR":         "4106902",
		"Curitiba, PR":          "4106902",
		"Curitiba":              "4106902",
		"sao paulo/SP":          "3550308",
		"SÃO PAULO/sp":          "3550308",
		"Brasilia":              "5300108",
		"4106902":               "4106902",
		"Alta Floresta D'Oeste": "1100015",
		"alta floresta doeste":  "1100015",
		"Alta Floresta D Oeste": "1100015",
		"  Curitiba/PR  ":       "4106902",
	}

	for entrada, querido := range casos {
		m, err := Resolver(entrada)
		if err != nil {
			t.Errorf("%q: %v", entrada, err)
			continue
		}
		if m.Codigo != querido {
			t.Errorf("%q devolveu %s (%s), esperava o codigo %s", entrada, m.Codigo, m, querido)
		}
	}
}

// TestResolverRecusaNomeAmbiguo is the case that matters most: 232 names
// repeat across states, and picking one would put the nota in the wrong city
// without the Sefin ever objecting.
func TestResolverRecusaNomeAmbiguo(t *testing.T) {
	_, err := Resolver("Bom Jesus")

	var ambiguo *ErroAmbiguo
	if !errors.As(err, &ambiguo) {
		t.Fatalf("esperava ErroAmbiguo, veio %v", err)
	}
	if len(ambiguo.Candidatos) < 2 {
		t.Fatalf("ErroAmbiguo com %d candidatos", len(ambiguo.Candidatos))
	}

	// The candidates have to be usable as the next attempt.
	for _, c := range ambiguo.Candidatos {
		resolvido, err := Resolver(c.String())
		if err != nil {
			t.Errorf("o candidato %q nao resolve: %v", c, err)
			continue
		}
		if resolvido.Codigo != c.Codigo {
			t.Errorf("o candidato %q resolveu para %s", c, resolvido.Codigo)
		}
	}

	// With the state it stops being ambiguous.
	if _, err := Resolver("Bom Jesus/PI"); err != nil {
		t.Errorf("com a UF deveria resolver: %v", err)
	}
}

func TestResolverNaoEncontrado(t *testing.T) {
	for _, entrada := range []string{"", "   ", "Xyzzy", "Curitiba/XX", "9999999"} {
		_, err := Resolver(entrada)

		var naoEncontrado *ErroNaoEncontrado
		if !errors.As(err, &naoEncontrado) {
			t.Errorf("%q: esperava ErroNaoEncontrado, veio %v", entrada, err)
		}
	}

	// A near miss should come back with something to try.
	_, err := Resolver("Curitba")
	var naoEncontrado *ErroNaoEncontrado
	if errors.As(err, &naoEncontrado) && len(naoEncontrado.Sugestoes) == 0 {
		t.Log("sem sugestoes para um erro de digitacao — aceitavel, a busca e por substring")
	}
}

func TestBuscar(t *testing.T) {
	// A prefix match ranks above a mere containment.
	resultados := Buscar("Curitiba", 10)
	if len(resultados) == 0 || resultados[0].Codigo != "4106902" {
		t.Errorf("busca por Curitiba trouxe %v", resultados)
	}

	// The state narrows it.
	for _, m := range Buscar("Bom Jesus/SC", 20) {
		if m.UF != "SC" {
			t.Errorf("a busca com UF trouxe %s", m)
		}
	}

	if got := Buscar("", 5); got != nil {
		t.Errorf("busca vazia devolveu %v", got)
	}
	if got := Buscar("xyzzy", 5); len(got) != 0 {
		t.Errorf("busca sem casamento devolveu %v", got)
	}
	if got := Buscar("São", 3); len(got) > 3 {
		t.Errorf("limite 3 devolveu %d", len(got))
	}
}
