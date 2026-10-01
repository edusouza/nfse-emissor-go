package rejeicao

import (
	"reflect"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/anexoi"
)

// TestTabelaEstaSincronizadaComOAnexo reads the annex and compares it with
// the committed table.
//
// Everything else here tests the table against itself, and would go on
// passing if someone had fixed a message by hand or the annex had been
// replaced without regenerating. Only the annex can say whether the table is
// right, and it is versioned in this repository.
func TestTabelaEstaSincronizadaComOAnexo(t *testing.T) {
	lidas, err := anexoi.Ler(anexoi.Caminho())
	if err != nil {
		t.Fatal(err)
	}

	esperado := make(map[string][]Regra, len(lidas))
	for _, r := range lidas {
		for _, regra := range r.Regras {
			esperado[r.Codigo] = append(esperado[r.Codigo], Regra{
				Mensagem: regra.Mensagem,
				Campo:    regra.Campo,
				Texto:    regra.Texto,
				Nivel:    regra.Nivel,
			})
		}
	}

	if !reflect.DeepEqual(esperado, anexoI) {
		for codigo := range esperado {
			if !reflect.DeepEqual(esperado[codigo], anexoI[codigo]) {
				t.Errorf("%s difere do anexo", codigo)
			}
		}
		for codigo := range anexoI {
			if _, ok := esperado[codigo]; !ok {
				t.Errorf("%s nao esta no anexo", codigo)
			}
		}
		t.Fatal("anexo_i.go nao e mais o que o anexo produz.\n" +
			"Se o anexo mudou, rode 'go generate ./internal/domain/rejeicao' e " +
			"registre a mudanca no CHANGELOG; o arquivo nao se edita a mao.")
	}
}

func TestBuscar(t *testing.T) {
	regras := Buscar("E0600", "")
	if len(regras) != 1 {
		t.Fatalf("E0600: %d regras, quer 1", len(regras))
	}
	r := regras[0]
	if r.CampoNaDPS() != "DPS/infDPS/valores/trib/tribMun/pAliq" {
		t.Errorf("E0600: campo na DPS %q", r.CampoNaDPS())
	}
	if r.DependeDoMunicipio() {
		t.Error("E0600 e regra geral, nivel 2")
	}
	if !r.AcrescentaAMensagem() {
		t.Error("a regra do E0600 diz opSimpNac = 2, que a mensagem nao diz")
	}

	// Lower case and stray spaces are still the same code.
	if len(Buscar(" e0600 ", "")) != 1 {
		t.Error("o codigo deveria ser encontrado sem depender de caixa e espacos")
	}

	if Buscar("E2240", "") != nil {
		t.Error("E2240 e do ADN (ANEXO IV), nao do ANEXO I")
	}
	if Buscar("", "") != nil {
		t.Error("codigo vazio nao deveria encontrar nada")
	}
}

func TestBuscarNivel3(t *testing.T) {
	regras := Buscar("E0635", "")
	if len(regras) != 1 || !regras[0].DependeDoMunicipio() {
		t.Fatalf("E0635 depende do convenio do municipio: %+v", regras)
	}
}

// The annex gives E1570 to two unrelated rules. The Sefin's sentence says
// which one was broken.
func TestBuscarCodigoRepetido(t *testing.T) {
	todas := Buscar("E1570", "")
	if len(todas) != 2 {
		t.Fatalf("E1570: %d regras, quer 2", len(todas))
	}

	// Wrapped, in another case and without the final period, as an answer
	// may come: still the same sentence.
	got := Buscar("E1570", "valor do diferimento  para a CBS\nnão deve ser informado")
	if len(got) != 1 {
		t.Fatalf("E1570 com a mensagem da CBS: %d regras, quer 1", len(got))
	}
	if got[0].CampoNaDPS() != "NFSe/infNFSe/IBSCBS/totCIBS/gCBS/vDifCBS" {
		t.Errorf("um campo que a Sefin calcula, fora da DPS, deveria manter o caminho inteiro: %q", got[0].CampoNaDPS())
	}

	if got := Buscar("E1570", "uma mensagem que o anexo nao tem"); len(got) != 2 {
		t.Errorf("sem saber qual regra, as duas deveriam vir: %d", len(got))
	}
}

func TestRecepcaoNaoTemCampo(t *testing.T) {
	regras := Buscar("E1200", "")
	if len(regras) == 0 {
		t.Fatal("E1200 ausente")
	}
	if regras[0].CampoNaDPS() != "" || regras[0].DependeDoMunicipio() {
		t.Errorf("regra da recepcao nao tem campo nem nivel: %+v", regras[0])
	}
}
