package servico

import (
	"strings"
	"testing"
)

func TestCodigoCompleto(t *testing.T) {
	casos := []struct {
		nome        string
		codigo      string
		complemento string
		quer        string
	}{
		{"cTribNac puro", "010701", "", "01.07.01.000"},
		{"cTribNac pontuado", "01.07.01", "", "01.07.01.000"},
		{"sem o zero a esquerda", "10701", "", "01.07.01.000"},
		{"com espacos em volta", "  010701 ", "", "01.07.01.000"},
		{"complemento informado", "010701", "001", "01.07.01.001"},
		{"codigo completo pontuado", "01.07.01.001", "", "01.07.01.001"},
		{"codigo completo sem pontos", "010701001", "", "01.07.01.001"},
		{"codigo completo sem o zero a esquerda", "10701001", "", "01.07.01.001"},
		{"completo e complemento iguais", "01.07.01.002", "002", "01.07.01.002"},
		{"item de dois digitos", "170101", "", "17.01.01.000"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			obtido, err := CodigoCompleto(c.codigo, c.complemento)
			if err != nil {
				t.Fatalf("CodigoCompleto(%q, %q) falhou: %v", c.codigo, c.complemento, err)
			}
			if obtido != c.quer {
				t.Errorf("CodigoCompleto(%q, %q) = %q, esperava %q", c.codigo, c.complemento, obtido, c.quer)
			}
		})
	}
}

func TestCodigoCompletoRecusa(t *testing.T) {
	casos := []struct {
		nome        string
		codigo      string
		complemento string
		trecho      string // part of the message that tells the user what to do
	}{
		{"vazio", "", "", "6 digitos"},
		{"letra no codigo", "01070a", "", "6 digitos"},
		{"digitos de menos", "0107", "", "6 digitos"},
		{"digitos demais", "0107010001", "", "6 digitos"},
		{"complemento curto", "010701", "1", "3 digitos"},
		{"complemento com letra", "010701", "00a", "3 digitos"},
		{"complemento divergente", "01.07.01.001", "002", "informe um so"},
		{"fora da lista nacional", "019999", "", "nfse servico"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			obtido, err := CodigoCompleto(c.codigo, c.complemento)
			if err == nil {
				t.Fatalf("CodigoCompleto(%q, %q) = %q, esperava erro", c.codigo, c.complemento, obtido)
			}
			if !strings.Contains(err.Error(), c.trecho) {
				t.Errorf("mensagem %q nao diz %q", err, c.trecho)
			}
		})
	}
}

// Every code in the national list has to turn into something the service
// accepts; this holds the format to the whole table rather than to examples.
func TestCodigoCompletoCobreALista(t *testing.T) {
	for _, s := range Todos() {
		obtido, err := CodigoCompleto(s.Codigo, "")
		if err != nil {
			t.Fatalf("%s: %v", s.Codigo, err)
		}
		if len(obtido) != 12 || strings.Count(obtido, ".") != 3 || !strings.HasSuffix(obtido, "."+ComplementoPadrao) {
			t.Errorf("%s virou %q, fora do formato II.SS.DD.CCC", s.Codigo, obtido)
		}
		if strings.ReplaceAll(obtido, ".", "")[:6] != s.Codigo {
			t.Errorf("%s virou %q, que nao comeca pelo cTribNac", s.Codigo, obtido)
		}
	}
}
