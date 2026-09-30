package texto

import (
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/anexoa"
)

func TestDobrar(t *testing.T) {
	casos := map[string]string{
		"São Paulo":         "sao paulo",
		"CONCEIÇÃO":         "conceicao",
		"Goiânia":           "goiania",
		"Itaú de Minas":     "itau de minas",
		"Pão de Açúcar":     "pao de acucar",
		"desenvolvimento 1": "desenvolvimento 1",
	}
	for entrada, esperado := range casos {
		if obtido := Dobrar(entrada); obtido != esperado {
			t.Errorf("Dobrar(%q) = %q, esperava %q", entrada, obtido, esperado)
		}
	}
}

func TestChave(t *testing.T) {
	iguais := [][]string{
		{"Alta Floresta D'Oeste", "alta floresta d oeste", "ALTA FLORESTA DOESTE", "Alta Floresta D’Oeste"},
		{"São Paulo", "sao paulo", "SAO PAULO", " são  paulo "},
		{"Embu-Guaçu", "embu guacu", "Embu Guaçu"},
	}
	for _, grupo := range iguais {
		for _, nome := range grupo[1:] {
			if Chave(nome) != Chave(grupo[0]) {
				t.Errorf("Chave(%q) = %q, mas Chave(%q) = %q", nome, Chave(nome), grupo[0], Chave(grupo[0]))
			}
		}
	}
	if Chave("São Paulo") == Chave("São Paulo do Potengi") {
		t.Error("nomes diferentes deram a mesma chave")
	}
}

// Dropping punctuation and spaces must not merge two municipalities of the
// same state, or a name typed on the command line would resolve to the wrong
// one. Across states names do repeat — that is what the UF is for.
func TestChaveNaoFundeMunicipiosDaMesmaUF(t *testing.T) {
	municipios, err := anexoa.Ler()
	if err != nil {
		t.Fatal(err)
	}

	visto := map[string]string{}
	for _, m := range municipios {
		chave := m.NomeUF + "/" + Chave(m.Nome)
		if outro, ok := visto[chave]; ok {
			t.Errorf("%q e %q, em %s, dao a mesma chave", outro, m.Nome, m.NomeUF)
		}
		visto[chave] = m.Nome

		// Every letter in the official names must fold: a character missing
		// from the table would silently drop out of the key.
		for _, r := range Dobrar(m.Nome) {
			if r > 127 && r != '’' {
				t.Errorf("%q tem %q, que Dobrar nao trata", m.Nome, r)
				break
			}
		}
	}
}
